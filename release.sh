#!/bin/bash
set -e
if [ $# -eq 0 ]; then
	VERSION=$(cat main.go|grep "VERSION string"| awk -v FS="(\")" '{print $2}')
else
  VERSION=$1
fi
echo Releasing $VERSION
WINF=whatscli-$VERSION-windows.zip
LINUXF=whatscli-$VERSION-linux.zip
MACF=whatscli-$VERSION-macos.zip
RASPIF=whatscli-$VERSION-raspberrypi.zip

GOOS=darwin go build -o whatscli
zip $MACF whatscli
rm whatscli
GOOS=windows go build -o whatscli.exe
zip $WINF whatscli.exe
rm whatscli.exe
GOOS=linux go build -o whatscli
zip $LINUXF whatscli
rm whatscli
GOOS=linux GOARCH=arm GOARM=5 go build -o whatscli
zip $RASPIF whatscli
rm whatscli

git pull
LASTTAG=$(git describe --tags --abbrev=0)
git log $LASTTAG..HEAD --no-decorate --pretty=format:"- %s" --abbrev-commit > changes.txt
vim changes.txt
gh release create $VERSION $LINUXF $MACF $WINF $RASPIF -F changes.txt -t $VERSION
rm changes.txt
rm *.zip
