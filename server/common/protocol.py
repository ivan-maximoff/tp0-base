import struct

OPCODE_BET = 0x01
OPCODE_ACK = 0x02
OPCODE_ERROR = 0x03
OPCODE_BATCH = 0x04
OPCODE_END_DATA = 0x05
OPCODE_GET_WINNERS = 0x06

HEADER_SIZE = 5

class Protocol:
    @staticmethod
    def send_frame(sock, opcode, body):
        # Header: 1 byte opcode + 4 bytes length (Big Endian '>')
        header = struct.pack('>BI', opcode, len(body))
        sock.sendall(header + body)

    @staticmethod
    def receive_frame(sock):
        # 1. Read Header
        header_raw = Protocol._recv_all(sock, HEADER_SIZE)
        if not header_raw: return None
        
        opcode, length = struct.unpack('>BI', header_raw)
        
        # 2. Read Body
        body = Protocol._recv_all(sock, length)
        return opcode, body

    @staticmethod
    def _recv_all(sock, n):
        """ Helper to handle Short Reads: keeps reading until n bytes are received """
        data = bytearray()
        while len(data) < n:
            packet = sock.recv(n - len(data))
            if not packet:
                return None # Connection closed
            data.extend(packet)
        return bytes(data)