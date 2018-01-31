#!/bin/bash -e
set -x #echo on

go version

rm -rf package
mkdir -p package/bin

cd ipms-importer
go build -x
VERSION=$(./ipms-importer -version | awk '{print $2}')
mv ipms-importer ../package/bin/ipms-importer-v${VERSION}-x86_64
cd ..

cd sqlite-importer
sudo docker run --rm -v $(pwd):$(pwd) -w $(pwd) --name "${PWD##*/}"-centos6 castis/centos6 /bin/bash -c "go get -v ./...; go build -x"
mv sqlite-importer ../package/bin/sqlite-importer-v${VERSION}-x86_64
cd ..

mkdir -p package/testtool
cd dummy-api-server
go build -x
mv dummy-api-server ../package/testtool/
cd ..

cp doc/glb-mapping.csv package/testtool/
cp doc/office-code-mapping.csv package/testtool/

mkdir -p package/doc
cp doc/ipms-importer.yml package/doc/
cp doc/sqlite-importer.yml package/doc/
cp doc/CHANGELOG.md package/doc/
scripts/md_to_pdf.py doc/CHANGELOG.md package/doc/CHANGELOG.pdf

mv package ipms-importer-v${VERSION}
tar cvzf ipms-importer-v${VERSION}.tar.gz ipms-importer-v${VERSION}

rm -rf ipms-importer-v${VERSION}
