import socket
import logging
import signal
import os
import threading
from common.protocol import Protocol, OPCODE_BET, OPCODE_ACK, OPCODE_ERROR, OPCODE_BATCH, OPCODE_END_DATA, OPCODE_GET_WINNERS
from common.bet import BetDeserializer
from common.utils import Bet, store_bets, load_bets, has_won

BETS_FILE = "bets.csv"
EXPECTED_BET_FIELDS = 6
DEFAULT_AGENCIES = 5

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

        self._lock = threading.Lock()
        self._total_agencies = int(os.getenv('CAN_AGENCIES', DEFAULT_AGENCIES))
        self._agencies_finished = set()
        self._lottery_done = False

        self._running = True
        signal.signal(signal.SIGTERM, self.__handle_signal)
        self.__cleanup_storage()

    def __cleanup_storage(self):
        """Removes the storage file if it exists."""
        if os.path.exists(BETS_FILE):
            os.remove(BETS_FILE)

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
        Main server loop that accepts connections and spawns threads.
        """
        while self._running:
            try:
                client_sock = self.__accept_new_connection()
                thread = threading.Thread(target=self.__handle_client_connection, args=(client_sock,))
                thread.start()
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
            while self._running:
                res = Protocol.receive_frame(client_sock)
                if not res:
                    break  # Client closed connection

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

            self.__safe_store_bets([bet])
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
                
                if consumed == 0 or len(fields) != EXPECTED_BET_FIELDS:
                    raise ValueError("Payload corrupto o incompleto")
                    
                bets.append(Bet(*fields))
                offset += consumed
            
            self.__safe_store_bets(bets)
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

        with self._lock:
            agency_id = body.decode()
            self._agencies_finished.add(agency_id)

            if len(self._agencies_finished) >= self._total_agencies and not self._lottery_done:
                logging.info("action: sorteo | result: success")
                self._lottery_done = True
        
        Protocol.send_frame(client_sock, OPCODE_ACK, b"")

    def __get_agency_winners(self, agency_id):
        """Logic to retrieve winning bets for a specific agency."""
        with self._lock:
            all_bets = load_bets()
            agency_winners = [
                bet.document for bet in all_bets
                if bet.agency == agency_id and has_won(bet)
            ]
        return agency_winners

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
        
        with self._lock:
            if not self._lottery_done:
                Protocol.send_frame(client_sock, OPCODE_ERROR, b"Sorteo no realizado")
                return

        winners_dni = self.__get_agency_winners(agency_id)
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

    def __safe_store_bets(self, bets):
        """
        Safely store bets in the database.
        """
        with self._lock:
            store_bets(bets)