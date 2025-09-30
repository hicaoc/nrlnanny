#!/bin/bash

workdir="/nrlnanny"
conf="$workdir/conf/nrlnanny.yaml"

if [ ! -f "$conf" ] ; then 
	cp $workdir/nrlnanny.yaml $workdir/conf/
fi


$workdir/nrlnanny -c $workdir/conf/nrlnanny.yaml

