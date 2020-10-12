FROM ubuntu:bionic

ENV namespace=""
ENV zkservers="zkserver:2181"
ENV zkpath="/rpc_v2"

COPY kube2zk /usr/local/bin/
ENTRYPOINT /usr/local/bin/kube2zk -namespace=$namespace -zkservers=$zkservers -zkpath=$zkpath
