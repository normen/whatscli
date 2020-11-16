#!/bin/bash
set -e
if [ $# -eq 0 ]; then
  echo "Usage: ./release.sh v1.0.0"
  exit 0
fi
WINF=whatscli-$1-windows.zip
LINUXF=whatscli-$1-linux.zip
MACF=whatscli-$1-macos.zip
RASPIF=whatscli-$1-raspberrypi.zip

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
echo "------ CHANGES ------"
cat changes.txt
echo "------ CHANGES ------"
gh release create $1 $LINUXF $MACF $WINF $RASPIF -F changes.txt -t $1
rm changes.txt
rm *.zip
