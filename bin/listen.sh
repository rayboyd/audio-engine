#!/bin/bash

# Simple UDP listener using netcat (nc) and hex dumper (xxd)
# Listens on port 9090 for UDP packets and displays their raw content.

PORT=9090
echo "Listening for UDP packets on port ${PORT}..."
echo "Packet structure expected (BigEndian):"
echo "  - Sequence (uint32)"
echo "  - Timestamp (int64)"
echo "  - Count (uint16)"
echo "  - Magnitudes (float32 array)"
echo "Press Ctrl+C to stop."
echo "---"

# -u: UDP mode
# -l: Listen mode
# -k: Keep listening after client disconnects (optional, useful if sender restarts)
# Pipe raw output to xxd for hex/ASCII view
nc -ulk ${PORT} | xxd -g 1 # -g 1 groups bytes individually
li
echo "---"
echo "Listener stopped."
