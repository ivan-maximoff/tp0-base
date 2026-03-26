import socket
import logging
import signal
import os
from common.protocol import Protocol, OPCODE_BET, OPCODE_ACK, OPCODE_ERROR, OPCODE_BATCH, OPCODE_END_DATA, OPCODE_GET_WINNERS
from common.bet import BetDeserializer
from common.utils import Bet, store_bets, load_bets, has_won

class Server:
    def __init__(self, port, listen_backlog):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        self._handlers = {
            OPCODE_BET: self.__handle_bet_message,
            OPCODE_BATCH: self.__handle_batch_message,
            OPCODE_END_DATA: self.__handle_end_data,
            OPCODE_GET_WINNERS: self.__handle_get_winners
        }
        self._total_agencies = int(os.getenv('CAN_AGENCIES', 5))
        self._agencies_finished = set()
        self._lottery_done = False

        self._running = True
        signal.signal(signal.SIGTERM, self.__handle_signal)

    def __handle_signal(self, signum, frame):
        """
        Signal handler for SIGTERM.
        Sets self._running to False and closes the main socket.
        """
        logging.info('action: signal_received | result: in_progress | signal: SIGTERM')
        self._running = False
        self._server_socket.close()
        logging.info('action: close_server_socket | result: success')

    def run(self):
        """
        Dummy Server loop

        Server that accept a new connections and establishes a
        communication with a client. After client with communucation
        finishes, servers starts to accept new connections again.

        The loop terminates gracefully if self._running is set to False via SIGTERM.
        """
        while self._running:
            try:
                client_sock = self.__accept_new_connection()
                self.__handle_client_connection(client_sock)
            except OSError:
                if not self._running:
                    logging.info('action: server_shutdown | result: success')
                    break

    def __handle_client_connection(self, client_sock):
        """
        Read message from a specific client socket and closes the socket

        If a problem arises in the communication with the client, the
        client socket will also be closed
        """
        try:
            res = Protocol.receive_frame(client_sock)
            if not res:
                return

            opcode, body = res
            handler = self._handlers.get(opcode)

            if handler:
                handler(client_sock, body)
            else:
                logging.error(f"action: receive_frame | result: fail | error: unknown_opcode {opcode}")
                Protocol.send_frame(client_sock, OPCODE_ERROR, b"Unknown Opcode")

        except Exception as e:
            logging.error(f"action: handle_client_connection | result: fail | error: {e}")
        finally:
            client_sock.close()

    def __handle_bet_message(self, client_sock, body):
        """Process a single bet"""
        try:
            fields = BetDeserializer.deserialize(body)
            bet = Bet(
                agency=fields[0],
                first_name=fields[1],
                last_name=fields[2],
                document=fields[3],
                birthdate=fields[4],
                number=fields[5]
            )

            store_bets([bet])
            logging.info(f'action: apuesta_almacenada | result: success | dni: {bet.document} | numero: {bet.number}')
            Protocol.send_frame(client_sock, OPCODE_ACK, b"")

        except Exception as e:
            logging.error(f"action: process_bet | result: fail | error: {e}")
            Protocol.send_frame(client_sock, OPCODE_ERROR, str(e).encode())

    def __handle_batch_message(self, client_sock, body):
        bets = []
        offset = 0
        try:
            while offset < len(body):
                if not self._running:
                    return
                fields, consumed = BetDeserializer.deserialize_single(body[offset:])
                bets.append(Bet(*fields))
                offset += consumed
            
            store_bets(bets)
            logging.info(f"action: apuesta_recibida | result: success | cantidad: {len(bets)}")
            Protocol.send_frame(client_sock, OPCODE_ACK, b"")
            
        except Exception as e:
            logging.error(f"action: apuesta_recibida | result: fail | cantidad: {len(bets)}")
            Protocol.send_frame(client_sock, OPCODE_ERROR, b"")

    def __handle_end_data(self, client_sock, body):
        """
        Handles the end of data notification from an agency.
        Triggers the lottery if the required number of agencies have finished.
        """

        agency_id = body.decode()
        self._agencies_finished.add(agency_id)

        if len(self._agencies_finished) >= self._total_agencies and not self._lottery_done:
            logging.info("action: sorteo | result: success")
            self._lottery_done = True
        
        Protocol.send_frame(client_sock, OPCODE_ACK, b"")

    def __handle_get_winners(self, client_sock, body):
        """
        Responds with the list of winning DNIs for the requesting agency.
        Only allowed after the lottery has been performed.
        """

        try:
            agency_id = int(body.decode())
        except ValueError:
            logging.error(f"Invalid agency id: {body.decode()}")
            Protocol.send_frame(client_sock, OPCODE_ERROR, b"ID invalido")
            return
        
        if not self._lottery_done:
            Protocol.send_frame(client_sock, OPCODE_ERROR, b"Sorteo no realizado")
            return

        all_bets = load_bets()
        winners_dni = []
        for bet in all_bets:
            if bet.agency == agency_id and has_won(bet):
                winners_dni.append(bet.document)
        
        response = ",".join(winners_dni).encode()
        Protocol.send_frame(client_sock, OPCODE_ACK, response)

    def __accept_new_connection(self):
        """
        Accept new connections

        Function blocks until a connection to a client is made.
        Then connection created is printed and returned
        """

        # Connection arrived
        logging.info('action: accept_connections | result: in_progress')
        c, addr = self._server_socket.accept()
        logging.info(f'action: accept_connections | result: success | ip: {addr[0]}')
        return c
