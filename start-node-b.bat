@echo off
rem P2PChain Node B - full node, seeds from A (use run-b data dir)
cd /d "%~dp0"
node.exe -listen 127.0.0.1:16690 -rpc 127.0.0.1:16691 -seed 127.0.0.1:6688 -datadir run-b
