#!/bin/bash

set -e

export CURL_HOST=$1
export SHASUM_HOST=$2
NODES=$3

for i in ${NODES//,/ }
do
  mkdir -p ./$i
done

vegeta attack -rate=1/s -lazy -format=json -duration=5m < <(
    while true; do
        export BODY=$(tr -d '\n' < <(echo "-Ls https://picsum.photos/1000 --output -" | curl -s -XPOST -H "Ipfaas-Publish-Ipfs: true" --data-binary @- http://$CURL_HOST/function/curl --output -))
        jq -ncM --arg SHASUM_HOST "$SHASUM_HOST" --arg BODY "$BODY" '{method: "POST", url: "http://\(env.SHASUM_HOST)/function/shasum", body: env.BODY | @base64, header: {"Content-Type": ["text/plain"], "Ipfaas-Is-Cid": ["true"]}}'
    done
) | \
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