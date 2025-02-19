#!/usr/bin/env bash

set -exu

git push --tags

TAG="$(git tag --sort=v:refname | tail -n1)"

docker run -it --rm -e HOME=/tmp/ -v "$(pwd)":/tmp/code -w /tmp/code golang:1.20-bullseye sh -c "git config --global --add safe.directory . && make reaction_${TAG:1}-1_amd64.deb reaction ip46tables nft46"

make "signatures_${TAG:1}"

rsync -avz -e 'ssh -J pica01' ./ip46tables ./nft46 ./reaction ./reaction_${TAG:1}-1_amd64.deb ./nft46.minisig ./ip46tables.minisig ./reaction.minisig ./reaction_${TAG:1}-1_amd64.deb.minisig akesi:/var/www/static/reaction/releases/"$TAG"

TOKEN="$(rbw get framagit.org token)"

DATA='{
"tag_name":"'"$TAG"'",
"description": "**Changes**\n\n**Download**\n\n```bash\nwget https://static.ppom.me/reaction/releases/'"$TAG"'/nft46 \\\n     https://static.ppom.me/reaction/releases/'"$TAG"'/reaction \\\n     https://static.ppom.me/reaction/releases/'"$TAG"'/ip46tables \\\n     https://static.ppom.me/reaction/releases/'"$TAG"'/nft46.minisig \\\n     https://static.ppom.me/reaction/releases/'"$TAG"'/reaction.minisig \\\n     https://static.ppom.me/reaction/releases/'"$TAG"'/ip46tables.minisig\nfor i in nft46 ip46tables reaction; do\n    minisign -VP RWSpLTPfbvllNqRrXUgZzM7mFjLUA7PQioAItz80ag8uU4A2wtoT2DzX -m $i &&\n      rm $i.minisig\ndone\n```\n\n**Debian Installation**\n\n```bash\nwget https://static.ppom.me/reaction/releases/'"$TAG"'/reaction_'"${TAG:1}"'-1_amd64.deb \\\n     https://static.ppom.me/reaction/releases/'"$TAG"'/reaction_'"${TAG:1}"'-1_amd64.deb.minisig\nminisign -VP RWSpLTPfbvllNqRrXUgZzM7mFjLUA7PQioAItz80ag8uU4A2wtoT2DzX -m reaction_'"${TAG:1}"'-1_amd64.deb &&\n  rm reaction_'"${TAG:1}"'-1_amd64.deb.minisig &&\n  apt install ./reaction_'"${TAG:1}"'-1_amd64.deb\n```",
"assets":{"links":[
{"url": "https://static.ppom.me/reaction/releases/'"$TAG"'/nft46", "name": "nft46 (x86-64)", "link_type": "package"},
{"url": "https://static.ppom.me/reaction/releases/'"$TAG"'/reaction", "name": "reaction (x86-64)", "link_type": "package"},
{"url": "https://static.ppom.me/reaction/releases/'"$TAG"'/ip46tables", "name": "ip46tables (x86-64)", "link_type": "package"},
{"url": "https://static.ppom.me/reaction/releases/'"$TAG"'/reaction_'"${TAG:1}"'-1_amd64.deb", "name": "reaction_'"${TAG:1}"'-1_amd64.deb (x86-64)", "link_type": "package"},
{"url": "https://static.ppom.me/reaction/releases/'"$TAG"'/nft46.minisig", "name": "nft46.minisig", "link_type": "other"},
{"url": "https://static.ppom.me/reaction/releases/'"$TAG"'/reaction.minisig", "name": "reaction.minisig", "link_type": "other"},
{"url": "https://static.ppom.me/reaction/releases/'"$TAG"'/ip46tables.minisig", "name": "ip46tables.minisig", "link_type": "other"},
{"url": "https://static.ppom.me/reaction/releases/'"$TAG"'/reaction_'"${TAG:1}"'-1_amd64.deb.minisig", "name": "reaction_'"${TAG:1}"'-1_amd64.deb.minisig", "link_type": "other"}
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
