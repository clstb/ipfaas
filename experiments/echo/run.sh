#!/bin/bash

export URL=$1
RATE=$2
NODES=$3

for i in ${NODES//,/ }
do
  mkdir -p ./$i
done

mkdir -p ./$PLATFORM

jq -ncM --arg URL "$URL" '{method: "POST", url: env.URL, body: "Hello world!" | @base64, header: {"Content-Type": ["text/plain"]}}' | \
  vegeta attack -duration=5m -rate=$RATE -format=json | \
  tee latencies.bin | \
  vegeta report -every=200ms &

while [[ -n $(jobs -r) ]]; do
  for i in ${NODES//,/ }
  do
    echo $[100-$(multipass exec $i -- vmstat 1 2|tail -1|awk '{print $15}')] >> ./$i/cpu-raw.txt;
    multipass exec $i -- free | sed -n '2p' | awk '{print $3}' >> ./$i/ram-raw.txt;
  done
  sleep 1;
done

for i in ${NODES//,/ }
do
  awk '{printf "%s,%s\n",NR,$1}' ./$i/cpu-raw.txt > ./$i/cpu.csv
  awk '{printf "%s,%s\n",NR,$1/1024}' ./$i/ram-raw.txt > ./$i/ram.csv
done
cat latencies.bin | vegeta report -type=hdrplot | awk 'NR>1 { printf "%s,%s\n", $2, $1}' > latencies.csv
