# USAGE: sh set_server_addr.sh <serverAddr> <miner-num-to-run>
# e.g. sh set_server_addr.sh 123.22.33.123 1
cp /usr/local/src/proj1_n0g9_p4d0b_r9i0b_s2d0b/supervisor/miner.conf /etc/supervisor/conf.d/miner$2.conf
sed -i "s/<server-addr>/$1/g" /etc/supervisor/conf.d/miner$2.conf
sed -i "s/<miner>/miner$2/g" /etc/supervisor/conf.d/miner$2.conf
sed -i "s/<miner-num>/$2/g" /etc/supervisor/conf.d/miner$2.conf
supervisorctl update miner$2