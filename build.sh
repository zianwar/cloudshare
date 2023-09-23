#!/bin/sh

agent_name="com.zianwar.cloudshare"

pid=$(launchctl list | grep -e "$agent_name" | awk '{print $1}')
if [ -n "$pid" ]; then
    echo "Agent already running"

    echo launchctl unload -w "$agent_name.plist"
    launchctl unload -w "$agent_name.plist"
fi

echo go build -o ~/.local/bin
go build -o ~/.local/bin
ls ~/.local/bin/cloudshare

echo launchctl load -w "$agent_name.plist"
launchctl load -w "$agent_name.plist"
