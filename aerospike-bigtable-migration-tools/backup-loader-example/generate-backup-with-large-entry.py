import msgpack
import base64
from sys import argv

if __name__ == "__main__":
    if (len(argv) != 2):
        raise Exception(f"USAGE: {argv[0]} NUM_ENTRIES")
    num_entries = int(argv[1])

    print("Version 3.1")
    print("# namespace test")
    print("# first-file")
    print("+ k S 4 key1")
    print("+ n test")
    print("+ d 7JEZLUt/jONdXXjTS8ply6qqyWA=")
    print("+ s demo")
    print("+ g 1")
    print("+ t 0")
    print("+ b 1")

    long_list = [i % 2 == 0 for i in range(num_entries)]
    packed_list_bytes = msgpack.packb(long_list)
    encoded_packed_list_bytes = base64.b64encode(packed_list_bytes).decode('ascii')
    print(f"- L longList {len(encoded_packed_list_bytes)} {encoded_packed_list_bytes}")
