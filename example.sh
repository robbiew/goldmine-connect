#!/bin/bash

# example, adjust for your BBS type/drop file location
export TERM=xterm

# passing node number only from Talisman
NODE_NUMBER=$1
TAG="SJK"           # set this to your BBS Tag 
# DOOR_CODE="MRC"   # comment this out if you want the Gold Mine Main Menu instead of a direct door code

# Define the path to the Talisman DOOR32.SYS file
DOOR32_SYS_PATH=/talisman/temp/${NODE_NUMBER}/door32.sys

# Extract the alias from line 7, replace spaces with underscores, and store it in a variable
USER_ALIAS=$(sed -n '7p' "$DOOR32_SYS_PATH" | tr ' ' '-')

# change to the directory where the goldmine-connect binary is located
cd /talsiman/doors/goldmine-connect

if [ -n "$DOOR_CODE" ]; then
    ./goldmine-connect -host goldminedoors.com -port 3513 -name $USER_ALIAS -tag $TAG -xtrn $DOOR_CODE 
else
    ./goldmine-connect -host goldminedoors.com -port 3513 -name $USER_ALIAS -tag $TAG
fi


