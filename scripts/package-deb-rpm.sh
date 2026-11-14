#!/usr/bin/env bash
# Build the coterie deb and rpm packages from a prebuilt linux binary.
#
# Usage: package-deb-rpm.sh <dist-dir> <goarch> <rpm-arch>
#   dist-dir  holds coterie-linux-<goarch> (raw binary, left in place)
#   goarch    GOARCH of that binary; also the deb Architecture field
#   rpm-arch  build target for the rpm (amd64 -> x86_64, arm64 -> aarch64)
#
# Packages land in <dist-dir> named with their own format's arch spelling,
# matching the existing coterie-nightly-linux-<goarch>.tar.gz assets:
# coterie-nightly-linux-<goarch>.deb and coterie-nightly-linux-<rpm-arch>.rpm.
# Both formats wrap the same staging tree: the server binary, the SQL
# migrations, a coterie.service unit, and an /etc/default/coterie template.
# The unit is not enabled automatically — it needs DATABASE_URL first.
set -euo pipefail

dist="$1"; goarch="$2"; rpm_arch="$3"
version="0.0.0~nightly"   # Debian versions must start with a digit
pkg="coterie"
maintainer="cuihairu <chuihairu@gmail.com>"
summary="Coterie - circle-based subscription server"
repo_root="$(cd "$(dirname "$0")/.." && pwd)"

binary="$dist/coterie-linux-$goarch"
if [ ! -x "$binary" ]; then
  echo "package: missing prebuilt $binary (build it first)" >&2
  exit 1
fi
for f in "$repo_root/migrations" "$repo_root/packaging/systemd/coterie.service" \
         "$repo_root/packaging/coterie.default"; do
  if [ ! -e "$f" ]; then
    echo "package: missing $f" >&2
    exit 1
  fi
done

stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT

root="$stage/root"
install -d -m 0755 \
  "$root/usr/bin" \
  "$root/usr/share/coterie/migrations" \
  "$root/usr/lib/systemd/system" \
  "$root/etc/default"
install -m 0755 "$binary" "$root/usr/bin/coterie"
install -m 0644 "$repo_root"/migrations/*.sql "$root/usr/share/coterie/migrations/"
install -m 0644 "$repo_root/packaging/systemd/coterie.service" \
  "$root/usr/lib/systemd/system/coterie.service"
install -m 0644 "$repo_root/packaging/coterie.default" "$root/etc/default/coterie"

# --- deb ---------------------------------------------------------------
mkdir "$root/DEBIAN"
cat >"$root/DEBIAN/control" <<EOF
Package: $pkg
Version: $version
Section: net
Priority: optional
Architecture: $goarch
Maintainer: $maintainer
Description: $summary
 Coterie serves the subscription API and the embedded web client from a
 single binary. The package installs the server, its SQL migrations, and
 a coterie.service systemd unit reading /etc/default/coterie.
EOF
printf '%s\n' '/etc/default/coterie' >"$root/DEBIAN/conffiles"
cat >"$root/DEBIAN/postinst" <<'EOF'
#!/bin/sh
echo "coterie: set DATABASE_URL in /etc/default/coterie, then: systemctl enable --now coterie"
EOF
chmod 0755 "$root/DEBIAN/postinst"
dpkg-deb --root-owner-group --build "$root" \
  "$dist/${pkg}-nightly-linux-${goarch}.deb"

# --- rpm ---------------------------------------------------------------
# No sources are compiled here, so the deb staging tree travels to
# rpmbuild as a single tarball and %install copies it into BUILDROOT.
rpmroot="$stage/rpm"
install -d "$rpmroot"/BUILD "$rpmroot"/RPMS "$rpmroot"/SOURCES \
  "$rpmroot"/SPECS "$rpmroot"/SRPMS "$rpmroot"/BUILDROOT
tar -C "$root" -czf "$rpmroot/SOURCES/${pkg}-nightly.tar.gz" usr etc
cat >"$rpmroot/SPECS/coterie.spec" <<EOF
# The Go binary is built with -ldflags "-s -w"; there is nothing for the
# debuginfo machinery to split out, and an empty debugfiles.list fails
# the build. Ship the package without a debuginfo companion.
%global debug_package %{nil}

Name:           $pkg
Version:        0.0.0
Release:        nightly%{?dist}
Summary:        $summary
License:        ASL 2.0
URL:            https://github.com/cuihairu/coterie
Source0:        ${pkg}-nightly.tar.gz
AutoReqProv:    no

%description
Coterie serves the subscription API and the embedded web client from a
single binary. The package installs the server, its SQL migrations, and
a coterie.service systemd unit reading /etc/default/coterie.

%prep
%setup -q -c -n coterie-root

%install
mkdir -p '%{buildroot}'
cp -a usr etc '%{buildroot}'

%files
%{_bindir}/coterie
/usr/share/coterie/migrations/*
/usr/lib/systemd/system/coterie.service
%config(noreplace) /etc/default/coterie

%post
echo "coterie: set DATABASE_URL in /etc/default/coterie, then: systemctl enable --now coterie"
EOF
rpmbuild -bb \
  --define "_topdir $rpmroot" \
  --target "$rpm_arch" \
  "$rpmroot/SPECS/coterie.spec"
mv "$rpmroot/RPMS/$rpm_arch"/*.rpm \
  "$dist/${pkg}-nightly-linux-${rpm_arch}.rpm"

echo "package: $dist/${pkg}-nightly-linux-${goarch}.deb"
echo "package: $dist/${pkg}-nightly-linux-${rpm_arch}.rpm"
