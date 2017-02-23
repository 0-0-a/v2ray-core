#!/bin/bash

set -x

apt-get update
apt-get -y install jq

function getattr() {
  curl -s -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/attributes/$1
}

GITHUB_TOKEN=$(getattr "github_token")
RELEASE_TAG=$(getattr "release_tag")
RELEASE_ID=$(curl -L -s https://api.github.com/repos/v2ray/v2ray-core/releases/tags/${RELEASE_TAG} | jq ".id")

mkdir -p /v2ray/build

curl -L -o /v2ray/build/releases https://api.github.com/repos/v2ray/v2ray-core/releases

GO_INSTALL=golang.tar.gz
curl -L -o ${GO_INSTALL} https://storage.googleapis.com/golang/go1.8.linux-amd64.tar.gz
tar -C /usr/local -xzf ${GO_INSTALL}
export PATH=$PATH:/usr/local/go/bin

mkdir -p /v2ray/src
export GOPATH=/v2ray

go get -u v2ray.com/core/...
go install v2ray.com/core/tools/build

$GOPATH/bin/build --os=windows --arch=x86 --zip
$GOPATH/bin/build --os=windows --arch=x64 --zip
$GOPATH/bin/build --os=macos --arch=x64 --zip
$GOPATH/bin/build --os=linux --arch=x86 --zip
$GOPATH/bin/build --os=linux --arch=x64 --zip
$GOPATH/bin/build --os=linux --arch=arm --zip
$GOPATH/bin/build --os=linux --arch=arm64 --zip
$GOPATH/bin/build --os=linux --arch=mips64 --zip
$GOPATH/bin/build --os=linux --arch=mips64le --zip
$GOPATH/bin/build --os=linux --arch=mips --zip
$GOPATH/bin/build --os=linux --arch=mipsle --zip
$GOPATH/bin/build --os=freebsd --arch=x86 --zip
$GOPATH/bin/build --os=freebsd --arch=amd64 --zip
$GOPATH/bin/build --os=openbsd --arch=x86 --zip
$GOPATH/bin/build --os=openbsd --arch=amd64 --zip

function upload() {
  FILE=$1
  CTYPE=$(file -b --mime-type $FILE)
  curl -H "Authorization: token ${GITHUB_TOKEN}" -H "Content-Type: ${CTYPE}" --data-binary @$FILE "https://uploads.github.com/repos/v2ray/v2ray-core/releases/${RELEASE_ID}/assets?name=$(basename $FILE)"
}

echo $(upload $GOPATH/bin/v2ray-macos.zip)
echo $(upload $GOPATH/bin/v2ray-windows-64.zip)
echo $(upload $GOPATH/bin/v2ray-windows-32.zip)
echo $(upload $GOPATH/bin/v2ray-linux-64.zip)
echo $(upload $GOPATH/bin/v2ray-linux-32.zip)
echo $(upload $GOPATH/bin/v2ray-linux-arm.zip)
echo $(upload $GOPATH/bin/v2ray-linux-arm64.zip)
echo $(upload $GOPATH/bin/v2ray-linux-mips64.zip)
echo $(upload $GOPATH/bin/v2ray-linux-mips64le.zip)
echo $(upload $GOPATH/bin/v2ray-linux-mips.zip)
echo $(upload $GOPATH/bin/v2ray-linux-mipsle.zip)
echo $(upload $GOPATH/bin/v2ray-freebsd-64.zip)
echo $(upload $GOPATH/bin/v2ray-freebsd-32.zip)
echo $(upload $GOPATH/bin/v2ray-openbsd-64.zip)
echo $(upload $GOPATH/bin/v2ray-openbsd-32.zip)
echo $(upload $GOPATH/bin/metadata.txt)



#INSTALL_DIR=_install

#git clone "https://github.com/v2ray/install.git" ${INSTALL_DIR}

#RELEASE_DIR=${INSTALL_DIR}/releases/${TRAVIS_TAG}
#mkdir -p ${RELEASE_DIR}/
#cp $GOPATH/bin/metadata.txt ${RELEASE_DIR}/
#cp $GOPATH/bin/v2ray-*.zip ${RELEASE_DIR}/
#echo ${TRAVIS_TAG} > ${INSTALL_DIR}/releases/latest.txt

#cp $GOPATH/bin/v2ray-${TRAVIS_TAG}-linux-64/v2ray ${INSTALL_DIR}/docker/official/

#pushd ${INSTALL_DIR}
#git config user.name "V2Ray Auto Build"
#git config user.email "admin@v2ray.com"
#git add -A
#git commit -m "Update for ${TRAVIS_TAG}"
#git push "https://${GIT_KEY_INSTALL}@github.com/v2ray/install.git" master
#popd

#DOCKER_HUB_API=https://registry.hub.docker.com/u/v2ray/official/trigger/${DOCKER_HUB_KEY}/
#curl -H "Content-Type: application/json" --data '{"build": true}' -X POST "${DOCKER_HUB_API}"
