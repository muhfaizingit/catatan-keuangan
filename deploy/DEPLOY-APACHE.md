# Panduan Deploy Manual — Keuangan Sekolah (Ubuntu + Apache + MySQL + PM2)

Target: VPS Ubuntu 22.04+, domain sudah di-A-record ke IP VPS.
Aplikasi Go dijalankan dengan **PM2** (process manager), di belakang **Apache**
(reverse proxy mod_proxy), HTTPS via certbot.
Port 8080 hanya diakses lokal oleh Apache — jangan dibuka ke publik.

Folder aplikasi: `/var/www/tkaba/keuangan`
Ganti `tk-aba19.oia.my.id` dengan domain Anda di semua langkah.

## 0. Prasyarat
- OS Ubuntu 22.04+ / Debian 12.
- DNS: A record `tk-aba19.oia.my.id` -> IP publik VPS.
- Firewall: buka 22, 80, 443. **Tutup 8080** (aplikasi bind ke 0.0.0.0:8080,
  jadi wajib ditutup di firewall agar tidak terekspos publik).

## 1. Install dependensi
```bash
sudo apt update
sudo apt install -y apache2 mysql-server git golang-go
go version   # butuh >= 1.21 (project 1.26). Kalau < 1.21, pasang dari go.dev/dl

# Node.js + PM2 (PM2 butuh Node untuk jalan, meski app-nya Go binary)
curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash -
sudo apt install -y nodejs
sudo npm install -g pm2
```

Aktifkan modul proxy Apache:
```bash
sudo a2enmod proxy proxy_http headers rewrite
sudo systemctl restart apache2
```

MySQL — buat DB + user (di VPS user app BOLEH punya hak penuh agar migrasi mulus):
```bash
sudo mysql
CREATE DATABASE keuangan CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'admin'@'localhost' IDENTIFIED BY 'PASSWORD_KUAT_DB';
GRANT ALL PRIVILEGES ON keuangan.* TO 'admin'@'localhost';
FLUSH PRIVILEGES;
exit
```

## 2. Ambil kode & build
```bash
sudo mkdir -p /var/www/tkaba/keuangan
sudo chown -R $USER:$USER /var/www/tkaba/keuangan
cd /var/www/tkaba/keuangan
git clone git@gitlab.com:cecod/catatan-keuangan.git .
# kalau sudah ada: git pull origin main

cd /var/www/tkaba/keuangan
go build -o /var/www/tkaba/keuangan/keuangan .
```
Binary: `/var/www/tkaba/keuangan/keuangan`. Folder `templates/` & `static/` harus tetap sejajar binary.

## 3. Environment produksi
```bash
cp deploy/.env.example /var/www/tkaba/keuangan/.env
nano /var/www/tkaba/keuangan/.env      # isi nilai nyata
```
- `APP_ENV=production` -> aktifkan gin.ReleaseMode.
- `JWT_SECRET` wajib kuat & unik (min 32 karakter).
- `APP_PORT=8080` (default, cocok dengan proxy Apache).
- PM2 tidak wajib membaca .env: app memakai `godotenv.Load()` yang otomatis
  membaca `.env` dari working directory (diatur oleh `cwd` di ecosystem config).

## 4. Jalankan dengan PM2
```bash
cd /var/www/tkaba/keuangan
pm2 start ecosystem.config.js
pm2 save                 # simpan daftar proses (biar persist)
pm2 startup              # generate systemd unit agar PM2 + app auto-start saat boot
sudo env PATH=$PATH:/usr/bin /usr/lib/node_modules/pm2/bin/pm2 startup systemd -u $USER --hp $HOME
```
Cek status:
```bash
pm2 status
pm2 logs keuangan        # lihat log (AutoMigrate + seed admin jalan di sini)
```
Pastikan status `online` dan tidak ada error koneksi DB.

Catatan: PM2 jalan sebagai user yang menjalankan perintah (bukan www-data).
Tidak masalah — Apache (www-data) hanya reverse-proxy ke `127.0.0.1:8080` secara lokal.

## 5. Apache virtual host (reverse proxy)
```bash
sudo cp deploy/apache-vhost.conf /etc/apache2/sites-available/tk-aba19.oia.my.id.conf
sudo a2ensite tk-aba19.oia.my.id.conf
# sudo a2dissite 000-default.conf   # opsional, nonaktifkan default
sudo apache2ctl configtest        # HARUS "Syntax OK"
sudo systemctl reload apache2
```

## 6. HTTPS (certbot)
```bash
sudo apt install -y certbot python3-certbot-apache
sudo certbot --apache -d tk-aba19.oia.my.id
```
Certbot otomatis set HTTPS + redirect 80->443 + renew otomatis.
Buka `https://tk-aba19.oia.my.id`.

## 7. Verifikasi
```bash
curl -I https://tk-aba19.oia.my.id/login     # 200
pm2 status keuangan
```
Login pakai `SEED_ADMIN_EMAIL` / `SEED_ADMIN_PASSWORD` dari `.env`.

## 8. Update aplikasi
```bash
cd /var/www/tkaba/keuangan
git pull origin main
go build -o /var/www/tkaba/keuangan/keuangan .
pm2 restart keuangan
```
(Apache tidak perlu di-restart saat update app, kecuali mengubah vhost.)

## Catatan
- File `deploy/keuangan.service` (systemd murni) TIDAK dipakai karena kita pakai PM2.
  Boleh diabaikan.
- Jangan pindah binary `keuangan` keluar `/var/www/tkaba/keuangan`.
- AutoMigrate jalan saat start -> tabel baru otomatis dibuat.
- Backup rutin: `mysqldump keuangan > backup.sql` (cron harian disarankan).
- Static assets dilayani oleh Go (`/static`); tidak perlu config static di Apache.
- Bila ganti domain, edit `ServerName` di vhost + jalankan ulang certbot dengan `-d domain.baru`.
