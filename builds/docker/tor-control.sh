#!/bin/bash

# Tor Control Script
# This script can be executed from within the Tor container to control Tor

ACTION=$1

case $ACTION in
    "newcircuit")
        echo "AUTHENTICATE" | nc 127.0.0.1 9051
        echo "SIGNAL NEWNYM" | nc 127.0.0.1 9051
        echo "Circuit rotated successfully"
        ;;
    "status")
        echo "AUTHENTICATE" | nc 127.0.0.1 9051
        echo "GETINFO version" | nc 127.0.0.1 9051
        ;;
    *)
        echo "Usage: $0 {newcircuit|status}"
        exit 1
        ;;
esac
