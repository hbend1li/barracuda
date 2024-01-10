#!/usr/bin/env bash

set -exu

docker run -it --rm -e HOME=/tmp/ -v "$(pwd)":/tmp/code -w /tmp/code -u "$(id -u)" golang:1.20 make reaction.deb

make signatures

TAG="$(git tag --sort=v:refname | tail -n1)"

rsync -avz -e 'ssh -J pica01' ./ip46tables ./reaction ./reaction.deb ./ip46tables.minisig ./reaction.minisig ./reaction.deb.minisig akesi:/var/www/static/reaction/releases/"$TAG"

TOKEN="$(rbw get framagit.org token)"

DATA='{
"tag_name":"'"$TAG"'",
"assets":{"links":[
{"url": "https://static.ppom.me/reaction/releases/'"$TAG"'/reaction", "name": "reaction (x86-64)", "link_type": "package"},
{"url": "https://static.ppom.me/reaction/releases/'"$TAG"'/ip46tables", "name": "ip46tables (x86-64)", "link_type": "package"},
{"url": "https://static.ppom.me/reaction/releases/'"$TAG"'/reaction.deb", "name": "reaction.deb (x86-64)", "link_type": "package"},
{"url": "https://static.ppom.me/reaction/releases/'"$TAG"'/reaction.minisig", "name": "reaction.minisig", "link_type": "other"},
{"url": "https://static.ppom.me/reaction/releases/'"$TAG"'/ip46tables.minisig", "name": "ip46tables.minisig", "link_type": "other"},
{"url": "https://static.ppom.me/reaction/releases/'"$TAG"'/reaction.deb.minisig", "name": "reaction.deb.minisig", "link_type": "other"}
]}}'

curl \
	--fail-with-body \
	--location \
	-X POST \
	-H 'Content-Type: application/json' \
	-H "PRIVATE-TOKEN: $TOKEN" \
	'https://framagit.org/api/v4/projects/90566/releases' \
	--data "$DATA"

make clean
