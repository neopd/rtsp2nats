#!/bin/sh

RTSP=rtsp://172.20.10.120:8554/h264ESVideoTest
TXQ=4
NR_CAM=25
NR_AREA=4

AREA=0
while [ $AREA -lt $NR_AREA ]; do
	./do-test.sh $RTSP $TXQ $AREA $NR_CAM &
	AREA=$((AREA+1))
done
