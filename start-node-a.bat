@echo off
rem P2PChain Node A - miner node (double-click to run)
rem Uses its own data dir run-a so it never collides with node B.
cd /d "%~dp0"
node.exe -listen 127.0.0.1:6688 -rpc 127.0.0.1:6689 -mine -miners 2 -datadir run-a
