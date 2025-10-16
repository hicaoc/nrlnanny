#!/bin/bash

workdir="/nrlnanny"
conf="$workdir/conf/nrlnanny.yaml"

if [ ! -f "$conf" ] ; then 
	cp $workdir/nrlnanny.yaml $workdir/conf/
fi

cd $workdir

$workdir/nrlnanny -c $workdir/conf/nrlnanny.yaml

