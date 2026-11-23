# Build and verify on Arch without changing the host distro or Docker workloads.
FROM archlinux:base AS builder
WORKDIR /build
COPY PKGBUILD arch-launcher.patch ARCH.md ./
RUN bash -c 'source PKGBUILD; pacman -Syu --needed --noconfirm base-devel namcap "${depends[@]}" "${makedepends[@]}"' \
    && useradd -m -u 1000 builder && chown -R builder:builder /build
USER builder
ENV PACKAGER="AxiomaAbsurdo <AxiomaAbsurdo@users.noreply.github.com>"
RUN makepkg --noconfirm \
    && makepkg --printsrcinfo > .SRCINFO \
    && makepkg --source --noconfirm \
    && namcap PKGBUILD *.pkg.tar.zst > namcap.log 2>&1

# Check dependency resolution in a separate image without build tools installed.
FROM archlinux:base AS runtime
COPY --from=builder /build/lazyconteiner-bin-*.pkg.tar.zst /tmp/
RUN sed -i '/^[[:space:]]*NoExtract[[:space:]]*=/d' /etc/pacman.conf \
    && pacman -Syu --noconfirm \
    && pacman -U --noconfirm /tmp/lazyconteiner-bin-*.pkg.tar.zst \
    && useradd -m -u 1000 builder
USER builder
RUN lazyconteiner --version && lazyconteiner --smoke-test

# Xvfb is only a verification dependency, not a package dependency.
FROM runtime AS check
USER root
RUN pacman -S --needed --noconfirm xorg-server-xvfb xorg-xwininfo imagemagick
COPY check-arch.py /opt/check-arch.py
USER builder
CMD ["python", "/opt/check-arch.py"]
