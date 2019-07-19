#!/bin/bash

SCRIPT=`realpath $0`
SCRIPTPATH=`dirname $SCRIPT`
BASENAME=`basename $SCRIPTPATH`
echo $BASENAME
kill `cat /tmp/$BASENAME-pid`
