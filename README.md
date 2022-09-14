# Introduction
This module impements a bridge which connects between RTSP client and NATS to test performance of video streaming using NATS
<br>
<br>
# Basic operation
An RTSP client based on gortsplib gets streaming data and publishes them to a certain 
topic of the NATS.
<br>
## Protocol to publish
`pub <topic> <bytes>\r\n<RAW data>`
<br>
## Topic format
`area:%d.cam:%d.%d`
<br>
<br>
# How to test
* Run nats-server with monitoring function
```
./nats-server -m 8222
```

* Run onDemandRTSPServer in live555/testProgs
```
./testOnDemandRTSPServer
```
* Run rtsp2nats
```
.\rtsp2nats.exe -txq 16 -url rtsp://172.20.10.120:8554/h264ESVideoTest
```