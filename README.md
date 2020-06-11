# Incognito Portal feeders

## BTC fullnode for BTC relayer
### How to run BTC fullnode
```
wget https://bitcoin.org/bin/bitcoin-core-0.20.0/bitcoin-0.20.0-x86_64-linux-gnu.tar.gz
tar -xvzf bitcoin-0.20.0-x86_64-linux-gnu.tar.gz
cd bitcoin-0.20.0/bin
cp bitcoind /usr/local/bin/

bitcoind -daemon=1 -conf=/{path_to}/bitcoin.conf -datadir=/{path_to_data}/bitcoin
```

### BTC fullnode config
```
listen=1
server=1
rpcuser=USERNAME
rpcpassword=PASSWORD
dbcache=4096
rpcallowip=0.0.0.0/0
blocknotify=/{path_to_event_script}/block.sh %s
[main]
port=8332
rpcport=18332
rpcbind=127.0.0.1
rpcbind=YOURIP
```
