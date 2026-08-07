#!/usr/bin/env bash
set -euo pipefail

if [ $# -ne 1 ]; then
  echo "Usage: $0 <version>" >&2
  exit 1
fi

version="$1"
pkgver="${version#v}"
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
stable_dir="$repo_root/.github/aur/whatscli"
git_dir="$repo_root/.github/aur/whatscli-git"
tarball_path="$repo_root/${version}.tar.gz"
git_pkgver="$(git -C "$repo_root" fetch --quiet --depth=1 origin '+HEAD:refs/remotes/origin/aur-head' && git -C "$repo_root" show -s --format='%cd.%h' --date=short refs/remotes/origin/aur-head | tr -d -)"

curl -fsSL "https://github.com/normen/whatscli/archive/${version}.tar.gz" -o "$tarball_path"
sha1="$(sha1sum "$tarball_path" | awk '{print $1}')"
rm -f "$tarball_path"

mkdir -p "$stable_dir" "$git_dir"

cat > "$stable_dir/PKGBUILD" <<EOF
# Maintainer: normen <normen@users.noreply.github.com>
pkgname=whatscli
pkgver=${pkgver}
pkgrel=1
pkgdesc='A command line interface for WhatsApp, based on go-whatsmeow and tview'
arch=('i686' 'x86_64' 'armv7h' 'armv6h' 'aarch64')
url='https://github.com/normen/whatscli'
makedepends=('go' 'git')
source=("\${pkgname}-\${pkgver}.tar.gz::https://github.com/normen/whatscli/archive/v\${pkgver}.tar.gz")
sha1sums=('${sha1}')

build() {
    cd "\${pkgname}-\${pkgver}"
    export CGO_CPPFLAGS="\${CPPFLAGS}"
    export CGO_CFLAGS="\${CFLAGS}"
    export CGO_CXXFLAGS="\${CXXFLAGS}"
    export CGO_LDFLAGS="\${LDFLAGS}"
    export GOFLAGS="-buildmode=pie -trimpath -mod=readonly -modcacherw"
    go build -o "\${pkgname}" -ldflags "-s -w -X main.VERSION=v\${pkgver}" .
}

package() {
    install -Dm755 "\${pkgname}-\${pkgver}/\${pkgname}" "\${pkgdir}/usr/bin/\${pkgname}"
}
EOF

cat > "$stable_dir/.SRCINFO" <<EOF
pkgbase = whatscli
	pkgdesc = A command line interface for WhatsApp, based on go-whatsmeow and tview
	pkgver = ${pkgver}
	pkgrel = 1
	url = https://github.com/normen/whatscli
	arch = i686
	arch = x86_64
	arch = armv7h
	arch = armv6h
	arch = aarch64
	makedepends = go
	makedepends = git
	source = whatscli-${pkgver}.tar.gz::https://github.com/normen/whatscli/archive/v${pkgver}.tar.gz
	sha1sums = ${sha1}

pkgname = whatscli
EOF

cat > "$git_dir/PKGBUILD" <<EOF
# Maintainer: normen <normen@users.noreply.github.com>
pkgname=whatscli-git
_pkgname=whatscli
pkgver=${git_pkgver}
pkgrel=1
pkgdesc='A command line interface for WhatsApp, based on go-whatsmeow and tview'
url='https://github.com/normen/whatscli'
arch=('i686' 'x86_64' 'armv7h')
makedepends=('git' 'go')
source=("git+\${url}.git")
sha256sums=('SKIP')

provides=("\${_pkgname}")
conflicts=("\${_pkgname}")

pkgver() {
	cd "\${srcdir}/\${_pkgname}"
	git log -1 --format='%cd.%h' --date=short | tr -d -
}

build() {
  cd "\${srcdir}/\${_pkgname}"
  export CGO_CPPFLAGS="\${CPPFLAGS}"
  export CGO_CFLAGS="\${CFLAGS}"
  export CGO_CXXFLAGS="\${CXXFLAGS}"
  export CGO_LDFLAGS="\${LDFLAGS}"
  export GOFLAGS="-buildmode=pie -trimpath -mod=readonly -modcacherw"
  go build -o "\${_pkgname}" -ldflags "-s -w -X main.VERSION=\${pkgver}" .
}

package() {
  install -Dm755 "\${srcdir}/\${_pkgname}/\${_pkgname}" "\${pkgdir}/usr/bin/\${_pkgname}"
}

# vim: ft=sh ts=2 sw=2 et
EOF

cat > "$git_dir/.SRCINFO" <<EOF
pkgbase = whatscli-git
	pkgdesc = A command line interface for WhatsApp, based on go-whatsmeow and tview
	pkgver = ${git_pkgver}
	pkgrel = 1
	url = https://github.com/normen/whatscli
	arch = i686
	arch = x86_64
	arch = armv7h
	makedepends = git
	makedepends = go
	provides = whatscli
	conflicts = whatscli
	source = git+https://github.com/normen/whatscli.git
	sha256sums = SKIP

pkgname = whatscli-git
EOF
