# Tayga wallpapers (source)

Backdrop artwork © **Алиса Кислова** · Apache-2.0  
Software © Кислов Роман Сергеевич

Repo source: `taiga/` and `cosmos/` with masters + sized JPEGs (desktop, retina laptop/tablet/phone).

## Distributable pack (separate)

Build/copy artifact (not embedded in the binary):

```
dist/tayga-wallpaper-pack-v0.4.0.zip
dist/wallpaper-pack/   # unpacked tree for inspection
```

Rebuild the zip:

```bash
# from repo root after assets/wallpapers is populated
rm -rf dist/wallpaper-pack
mkdir -p dist/wallpaper-pack
for theme in taiga cosmos; do
  mkdir -p "dist/wallpaper-pack/$theme"
  for f in desktop-1080p desktop-1440p desktop-4k desktop-5k \
           laptop-16x10 mbp14-retina mbp16-retina \
           ipad-retina ipad-pro-retina \
           iphone-13-retina iphone-15-pro-retina android-qhd-plus; do
    cp "assets/wallpapers/$theme/${f}.jpg" "dist/wallpaper-pack/$theme/"
  done
done
cp assets/wallpapers/README.md dist/wallpaper-pack/README.md
(cd dist && zip -qr tayga-wallpaper-pack-v0.4.0.zip wallpaper-pack)
```

UI embeds only `internal/frontend/dist/assets/taiga-forest[.jpg|@2x.jpg]` and `cosmic-abstract[.jpg|@2x.jpg]`.
