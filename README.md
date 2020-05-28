# pkg
Golang packages that support Tokenized repositories as well as Bitcoin and related functionality.

# Packages


+ bitcoin - Bitcoin key and function implementations.
+ bsvalias - client implementation of bsvalias/paymail.
+ json - a "better" json implementation that uses hex instead of base64 for binary fields.
+ logger - an upgraded logging system using context passing and objects.
+ rpcnode - a client implementation for interacting with a full Bitcoin node to retrieve data.
+ scheduler - a simple task scheduler.
+ spynode - a non-full node that can monitor the chain for related transactions and double spend attempts.
+ storage - a versatile data storage interface supporting local file storage and Amazon S3.
+ txbuilder - functionality for building Bitcoin transactions.
+ wire - an implementation of the Bitcoin P2P messages.
