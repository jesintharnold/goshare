openssl genrsa -out peerA.key 2048
openssl req -new -key peerA.key -x509 -days 365 -out peerA.crt -subj "/CN=PeerA"

openssl genrsa -out peerB.key 2048
openssl req -new -key peerB.key -x509 -days 365 -out peerB.crt -subj "/CN=PeerB"

openssl x509 -in peerA.crt -text -noout
openssl x509 -in peerB.crt -text -noout
