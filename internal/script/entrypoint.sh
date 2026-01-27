#!/bin/sh

export CRI_RUNTIME_ENDPOINT="unix:///run/containerd/containerd.sock"

echo "Starting sniffer for container: $CONTAINER_ID"

PID=$(crictl inspect -o json "$CONTAINER_ID" 2>/dev/null | jq -r '.info.pid')

if [ -z "$PID" ] || [ "$PID" == "null" ]; then
    echo "ERROR: Could not find PID for container $CONTAINER_ID"
    exit 1
fi

echo "Found PID: $PID. Starting capture..."

timeout "$DURATION" nsenter --net=/host/proc/$PID/ns/net tcpdump -i any -U -w /data/dump.pcap "$FILTER"

touch /data/capture_finished

echo "Capture complete. Dump saved to /data/dump.pcap"