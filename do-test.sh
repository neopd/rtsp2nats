#!/bin/sh

RTSP=$1
TXQ=$2
AREA=$3
NR_CAM=$4

CAMID=0
while [ $CAMID -lt $NR_CAM ]; do
	./rtsp2nats -txq $TXQ -url $RTSP -subject area.$AREA.cam.$CAMID.0 &
	CAMID=$((CAMID+1))
done
