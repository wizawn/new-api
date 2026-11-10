#!/usr/bin/env python3
"""仅在已有隔离命名空间内维护 New API 自有链，不刷新生产防火墙。"""
import json
import subprocess

NFT = ["nsenter", "--net=/run/netns/redeemai-rehearsal", "--", "nft"]
state = json.loads(subprocess.check_output(NFT + ["-j", "list", "table", "inet", "rehearsal_guard"]))
chains = {item["chain"]["name"] for item in state["nftables"] if "chain" in item}
rules = [item["rule"] for item in state["nftables"] if "rule" in item]
commands = []
for parent in ("output", "forward"):
    child = "newapi_router_" + parent
    if child not in chains:
        commands.append(f"add chain inet rehearsal_guard {child}")
    commands.append(f"flush chain inet rehearsal_guard {child}")
    if not any(rule.get("comment") == "newapi-router-" + parent for rule in rules):
        commands.append(f'insert rule inet rehearsal_guard {parent} jump {child} comment "newapi-router-{parent}"')

commands += [
    'add rule inet rehearsal_guard newapi_router_output oifname "newapi-out" ip daddr 10.206.60.10 tcp dport 3000 accept',
    'add rule inet rehearsal_guard newapi_router_output oifname "newapi-out" ip daddr 10.206.60.11 tcp dport 80 accept',
    'add rule inet rehearsal_guard newapi_router_forward iifname "newapi-db" oifname "newapi-db" ip saddr 10.206.61.0/24 ip daddr 10.206.61.0/24 accept',
    'add rule inet rehearsal_guard newapi_router_forward iifname "newapi-out" oifname "newapi-out" ip saddr 10.206.60.0/24 ip daddr 10.206.60.0/24 accept',
    'add rule inet rehearsal_guard newapi_router_forward iifname "rmg-test" oifname "newapi-out" ct state established,related accept',
    'add rule inet rehearsal_guard newapi_router_forward iifname "newapi-out" oifname "rmg-test" ip saddr { 10.206.60.10, 10.206.60.11 } ip daddr != { 0.0.0.0/8, 10.0.0.0/8, 127.0.0.0/8, 169.254.0.0/16, 172.16.0.0/12, 192.168.0.0/16, 224.0.0.0/4 } accept',
]
payload = "\n".join(commands) + "\n"
subprocess.run(NFT + ["--check", "-f", "-"], input=payload, text=True, check=True)
subprocess.run(NFT + ["-f", "-"], input=payload, text=True, check=True)
