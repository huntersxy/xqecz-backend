"""banned_words encoding tool - run once when words change"""
import json, sys, os
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_DIR = os.path.dirname(SCRIPT_DIR)

KEY = 0x5A  # fixed XOR key

def encode(path_in: str, path_out: str) -> int:
    with open(path_in, "r", encoding="utf-8") as f:
        data = json.load(f)
    words = data.get("words", [])
    count = len(words)
    size = 4  # uint32 count prefix
    for w in words:
        b = w.encode("utf-8")
        size += 2 + len(b)  # uint16 length + bytes
    buf = bytearray(size)
    buf[0:4] = count.to_bytes(4, "little")
    pos = 4
    for w in words:
        wb = w.encode("utf-8")
        buf[pos:pos+2] = len(wb).to_bytes(2, "little")
        pos += 2
        for ch in wb:
            buf[pos] = ch ^ KEY
            pos += 1
    with open(path_out, "wb") as f:
        f.write(buf)
    return count

if __name__ == "__main__":
    src = os.path.join(PROJECT_DIR, "config", "banned_words.json")
    dst = os.path.join(PROJECT_DIR, "config", "banned_words.bin")
    if not os.path.exists(src):
        print("banned_words.json not found, skipping")
        sys.exit(0)
    n = encode(src, dst)
    print(f"Encoded {n} words -> {dst}")
