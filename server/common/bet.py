import struct

class BetDeserializer:
    @staticmethod
    def deserialize(data):
        """ 
        Reconstructs fields from bytes using length prefixes.
        Expected order: Agency, Name, LastName, DNI, Birth, Number
        """
        fields = []
        offset = 0
        
        for _ in range(6):
            if offset + 2 > len(data):
                break
                
            # Read 2 bytes for field length (Big Endian unsigned short)
            field_len = struct.unpack('>H', data[offset:offset+2])[0]
            offset += 2
            
            # Read the field content
            field_val = data[offset:offset+field_len].decode('utf-8')
            fields.append(field_val)
            offset += field_len
            
        return fields