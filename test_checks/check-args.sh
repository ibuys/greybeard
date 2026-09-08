#!/bin/sh
# Usage: ./check.sh <warning_threshold> <critical_threshold>

WARN="$1"
CRIT="$2"

if [ -z "$WARN" ] || [ -z "$CRIT" ]; then
    echo "UNKNOWN - Usage: $0 <warning> <critical>"
    exit 3
fi

VALUE=42

if [ "$VALUE" -ge "$CRIT" ]; then
    echo "CRITICAL - value is $VALUE | value=$VALUE;$WARN;$CRIT"
    exit 2
elif [ "$VALUE" -ge "$WARN" ]; then
    echo "WARNING - value is $VALUE | value=$WARN;$WARN;$CRIT"
    exit 1
else
    echo "OK - everything is fine | value=$VALUE;$WARN;$CRIT"
    exit 0
fi