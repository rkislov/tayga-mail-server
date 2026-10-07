# Tayga wallpapers (source)

Backdrop artwork © **Алиса Кислова** · Apache-2.0  
Software © Кислов Роман Сергеевич

Themes in this tree:

| Dir | Theme | Subject |
|-----|-------|---------|
| `taiga/` | Тайга (default UI) | northern forest |
| `cosmos/` | Космос | abstract space |
| `city/` | Город | riverside city at dusk |
| `kalyazin/` | Колязин | flooded bell tower |
| `temple/` | Храм на Нерли | Church of the Intercession on the Nerl |
| `moscow/` | Москва-Сити | Moscow City skyline |
| `street-art/` | Стрит-арт | mural ad for Tayga Mail Server |

Each theme folder has masters (`master-16x9`, `master-4x3`, `master-9x16`, `source-author`) plus sized JPEGs:

- desktop-1080p / 1440p / 4k / 5k
- laptop-16x10, mbp14-retina, mbp16-retina
- ipad-retina, ipad-pro-retina
- iphone-13-retina, iphone-15-pro-retina, android-qhd-plus

## Distributable pack

```bash
# from repo root
rm -rf dist/wallpaper-pack
mkdir -p dist/wallpaper-pack
for theme in taiga cosmos city kalyazin temple moscow street-art; do
  mkdir -p "dist/wallpaper-pack/$theme"
  for f in desktop-1080p desktop-1440p desktop-4k desktop-5k \
           laptop-16x10 mbp14-retina mbp16-retina \
           ipad-retina ipad-pro-retina \
           iphone-13-retina iphone-15-pro-retina android-qhd-plus; do
    cp "assets/wallpapers/$theme/${f}.jpg" "dist/wallpaper-pack/$theme/"
  done
done
cp assets/wallpapers/README.md dist/wallpaper-pack/README.md
(cd dist && zip -qr tayga-wallpaper-pack.zip wallpaper-pack)
```

UI embeds 1x/2x under `internal/frontend/dist/assets/` (`taiga-forest`, `cosmic-abstract`, `city`, `kalyazin`, `temple-nerl`, `moscow-city`, `street-art`).
