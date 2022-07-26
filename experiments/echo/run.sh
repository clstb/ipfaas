#!/bin/bash

set -e

export HOST=$1
RATE=$2
NODES=$3

for i in ${NODES//,/ }
do
  mkdir -p ./$i
done

jq --arg HOST "$HOST" -ncM 'while(true; .+1) | {method: "POST", url: "http://\(env.HOST)/function/echo", body: . | @base64, header: {"Content-Type": ["text/plain"]}}' | \
vegeta attack -lazy --format=json -duration=2m -rate $RATE | tee latencies.bin | vegeta report -every=200ms &

while [[ -n $(jobs -r) ]]; do
  for i in ${NODES//,/ }
  do
    echo $[100-$(multipass exec $i -- vmstat 1 2|tail -1|awk '{print $15}')] >> ./$i/cpu-raw.txt;
    multipass exec $i -- free | sed -n '2p' | awk '{print $3}' >> ./$i/ram-raw.txt;
  done
done

for i in ${NODES//,/ }
do
  awk '{printf "%s,%s\n",NR,$1}' ./$i/cpu-raw.txt > ./$i/cpu.csv
  awk '{printf "%s,%s\n",NR,$1/1024}' ./$i/ram-raw.txt > ./$i/ram.csv
done
cat latencies.bin | vegeta report -type=hdrplot | awk 'NR>1 { printf "%s,%s\n", $2, $1}' > latencies.csv
