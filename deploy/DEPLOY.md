# Panduan Deploy — Keuangan Sekolah (tanpa Docker)

Target: VPS Ubuntu/Debian, domain `tk-aba19.oia.my.id` sudah di-A-record ke IP VPS.
Aplikasi dijalankan sebagai systemd service, di-depan Nginx (reverse proxy) + HTTPS (certbot).
Port 8080 hanya diakses lokal oleh Nginx — jangan dibuka ke publik.

## 0. Prasyarat
- OS Ubuntu 22.04+ / Debian 12.
- DNS: A record `tk-aba19.oia.my.id` → IP publik VPS.
- Firewall buka 22, 80, 443 (tutup 8080).

## 1. Install dependensi
```
sudo apt update
sudo apt install -y nginx mysql-server git golang-go
go version   # butuh >= 1.21 (project 1.26). Kalau < 1.21, pasang dari go.dev/dl
```

MySQL — buat DB + user (di VPS user app BOLEH punya hak CREATE INDEX agar migrasi lebih mulus):
```
sudo mysql
CREATE DATABASE keuangan CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'admin'@'localhost' IDENTIFIED BY 'PASSWORD_KUAT_DB';
GRANT ALL PRIVILEGES ON keuangan.* TO 'admin'@'localhost';
FLUSH PRIVILEGES;
exit
```

## 2. Ambil kode & build
```
sudo mkdir -p /opt/keuangan
sudo chown -R $USER:$USER /opt/keuangan
cd /opt/keuangan
git clone git@gitlab.com:cecod/catatan-keuangan.git .
# kalau sudah ada: git pull origin main

cd /opt/keuangan
go build -o /opt/keuangan/keuangan .
```
Binary: `/opt/keuangan/keuangan`. Folder `templates/` & `static/` harus tetap sejajar binary.

## 3. Environment produksi
```
sudo cp deploy/.env.example /opt/keuangan/.env
sudo nano /opt/keuangan/.env      # isi nilai nyata
```
- `APP_ENV=production` → aktifkan gin.ReleaseMode.
- `JWT_SECRET` wajib kuat & unik.

## 4. Systemd service
```
sudo cp deploy/keuangan.service /etc/systemd/system/keuangan.service
sudo chown -R www-data:www-data /opt/keuangan
sudo chmod 640 /opt/keuangan/.env
sudo chmod 750 /opt/keuangan
sudo chmod +x /opt/keuangan/keuangan

sudo systemctl daemon-reload
sudo systemctl enable --now keuangan
sudo systemctl status keuangan     # harus active (running)
```
Log: `sudo journalctl -u keuangan -f`

`WorkingDirectory=/opt/keuangan` PENTING agar path relatif
`templates/**` dan `./static` tetap ketemu.

## 5. Nginx virtual host
```
sudo cp deploy/nginx-vhost.conf /etc/nginx/sites-available/tk-aba19.oia.my.id
sudo ln -s /etc/nginx/sites-available/tk-aba19.oia.my.id /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

## 6. HTTPS (certbot)
```
sudo apt install -y certbot python3-certbot-nginx
sudo certbot --nginx -d tk-aba19.oia.my.id
```
Certbot otomatis set HTTPS + redirect 80→443 + renew otomatis.
Buka `https://tk-aba19.oia.my.id`.

## 7. Verifikasi
```
curl -I https://tk-aba19.oia.my.id/login     # 200
sudo systemctl status keuangan
```
Login pakai `SEED_ADMIN_EMAIL` / `SEED_ADMIN_PASSWORD`.

## 8. Update aplikasi
```
cd /opt/keuangan
git pull origin main
go build -o /opt/keuangan/keuangan .
sudo systemctl restart keuangan
```

## Catatan
- Jangan pindah binary `keuangan` keluar `/opt/keuangan`.
- `AutoMigrate` jalan saat start → tabel baru otomatis dibuat.
- Backup rutin: `mysqldump keuangan > backup.sql` (cron harian disarankan).
- Static assets dilayani oleh Go (`/static`); tidak perlu config static di Nginx.
