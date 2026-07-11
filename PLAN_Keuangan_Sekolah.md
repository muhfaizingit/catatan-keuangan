# Plan Aplikasi Keuangan Sekolah
> Dibuat: Juni 2026 | Status: Draft Plan

---

## 1. Ringkasan Proyek

Aplikasi pencatatan keuangan sekolah berbasis web, dibangun dengan Golang + Gin sebagai backend sekaligus server HTML (monolith). Database menggunakan MySQL. Dirancang agar API-nya bisa dipakai ulang oleh aplikasi mobile di masa depan tanpa perubahan backend.

---

## 2. Tech Stack

| Komponen | Teknologi | Keterangan |
|---|---|---|
| Language | Go (Golang) | Backend + serve HTML |
| Framework | Gin | HTTP router, middleware |
| Database | MySQL | Penyimpanan data utama |
| Auth | JWT | Login, role-based access |
| Template | html/template (bawaan Go) | Render halaman web |
| Interaktivitas | HTMX | Update partial tanpa reload |
| Frontend CSS | Tailwind CSS (CDN) | Styling ringan |
| Export | gopdf + excelize | Export PDF & Excel |

---

## 3. Struktur Folder Project

```
school-finance/
├── main.go
├── config/
│   └── config.go            # DB connection, env
├── middleware/
│   ├── auth.go              # JWT validation
│   └── rbac.go              # Role-based access check
├── models/
│   ├── user.go
│   ├── siswa.go
│   ├── kelas.go
│   ├── tahun_ajaran.go
│   ├── jenis_pemasukan.go
│   ├── kategori_pengeluaran.go
│   ├── kas_pemasukan.go
│   ├── kas_pengeluaran.go
│   ├── spp.go
│   ├── tabungan.go
│   └── setting.go
├── handlers/
│   ├── auth_handler.go
│   ├── dashboard_handler.go
│   ├── master_handler.go
│   ├── kas_handler.go
│   ├── spp_handler.go
│   ├── tabungan_handler.go
│   ├── tutup_tahun_handler.go
│   ├── laporan_handler.go
│   └── setting_handler.go
├── routes/
│   └── routes.go
├── templates/
│   ├── layout/
│   │   ├── base.html
│   │   └── sidebar.html
│   ├── auth/login.html
│   ├── dashboard/index.html
│   ├── master/
│   ├── kas/
│   ├── spp/
│   ├── tabungan/
│   ├── laporan/
│   └── setting/
├── static/
│   ├── css/
│   ├── js/
│   └── img/
└── go.mod
```

---

## 4. Desain Database (Skema MySQL)

### Tabel: users
```sql
CREATE TABLE users (
  id         BIGINT AUTO_INCREMENT PRIMARY KEY,
  nama       VARCHAR(100) NOT NULL,
  email      VARCHAR(100) NOT NULL UNIQUE,
  password   VARCHAR(255) NOT NULL,
  role       ENUM('admin', 'bendahara', 'kepala_sekolah') NOT NULL,
  aktif      TINYINT(1) DEFAULT 1,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

### Tabel: tahun_ajaran
```sql
CREATE TABLE tahun_ajaran (
  id         BIGINT AUTO_INCREMENT PRIMARY KEY,
  nama       VARCHAR(20) NOT NULL,   -- contoh: "2024/2025"
  aktif      TINYINT(1) DEFAULT 0,   -- hanya 1 yang aktif
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### Tabel: kelas
```sql
CREATE TABLE kelas (
  id              BIGINT AUTO_INCREMENT PRIMARY KEY,
  nama            VARCHAR(50) NOT NULL,
  tahun_ajaran_id BIGINT NOT NULL,
  FOREIGN KEY (tahun_ajaran_id) REFERENCES tahun_ajaran(id)
);
```

### Tabel: siswa
```sql
CREATE TABLE siswa (
  id         BIGINT AUTO_INCREMENT PRIMARY KEY,
  nis        VARCHAR(20) NOT NULL UNIQUE,
  nama       VARCHAR(100) NOT NULL,
  kelas_id   BIGINT,
  aktif      TINYINT(1) DEFAULT 1,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (kelas_id) REFERENCES kelas(id)
);
```

### Tabel: jenis_pemasukan (dimaster, bisa tambah bebas)
```sql
CREATE TABLE jenis_pemasukan (
  id         BIGINT AUTO_INCREMENT PRIMARY KEY,
  nama       VARCHAR(100) NOT NULL,
  keterangan TEXT,
  aktif      TINYINT(1) DEFAULT 1,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
-- Contoh data: Dana Dinas, Dana Yayasan, Pembayaran Orang Tua, Sumbangan, dst.
```

### Tabel: kategori_pengeluaran
```sql
CREATE TABLE kategori_pengeluaran (
  id         BIGINT AUTO_INCREMENT PRIMARY KEY,
  nama       VARCHAR(100) NOT NULL,
  aktif      TINYINT(1) DEFAULT 1,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### Tabel: sub_kategori_pengeluaran
```sql
CREATE TABLE sub_kategori_pengeluaran (
  id          BIGINT AUTO_INCREMENT PRIMARY KEY,
  kategori_id BIGINT NOT NULL,
  nama        VARCHAR(100) NOT NULL,
  aktif       TINYINT(1) DEFAULT 1,
  FOREIGN KEY (kategori_id) REFERENCES kategori_pengeluaran(id)
);
```

### Tabel: kas_pemasukan
```sql
CREATE TABLE kas_pemasukan (
  id                 BIGINT AUTO_INCREMENT PRIMARY KEY,
  tahun_ajaran_id    BIGINT NOT NULL,
  jenis_pemasukan_id BIGINT NOT NULL,
  tanggal            DATE NOT NULL,
  jumlah             DECIMAL(15,2) NOT NULL,
  keterangan         TEXT,
  user_id            BIGINT NOT NULL,
  created_at         DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (tahun_ajaran_id) REFERENCES tahun_ajaran(id),
  FOREIGN KEY (jenis_pemasukan_id) REFERENCES jenis_pemasukan(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
```

### Tabel: kas_pengeluaran
```sql
CREATE TABLE kas_pengeluaran (
  id              BIGINT AUTO_INCREMENT PRIMARY KEY,
  tahun_ajaran_id BIGINT NOT NULL,
  kategori_id     BIGINT NOT NULL,
  sub_kategori_id BIGINT,
  tanggal         DATE NOT NULL,
  jumlah          DECIMAL(15,2) NOT NULL,
  keterangan      TEXT,
  user_id         BIGINT NOT NULL,
  created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (tahun_ajaran_id) REFERENCES tahun_ajaran(id),
  FOREIGN KEY (kategori_id) REFERENCES kategori_pengeluaran(id),
  FOREIGN KEY (sub_kategori_id) REFERENCES sub_kategori_pengeluaran(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
```

### Tabel: spp_tagihan
```sql
CREATE TABLE spp_tagihan (
  id              BIGINT AUTO_INCREMENT PRIMARY KEY,
  siswa_id        BIGINT NOT NULL,
  tahun_ajaran_id BIGINT NOT NULL,
  bulan           TINYINT NOT NULL,   -- 1-12
  jumlah          DECIMAL(15,2) NOT NULL,
  status          ENUM('belum', 'lunas', 'cicil') DEFAULT 'belum',
  FOREIGN KEY (siswa_id) REFERENCES siswa(id),
  FOREIGN KEY (tahun_ajaran_id) REFERENCES tahun_ajaran(id)
);
```

### Tabel: spp_pembayaran
```sql
CREATE TABLE spp_pembayaran (
  id           BIGINT AUTO_INCREMENT PRIMARY KEY,
  tagihan_id   BIGINT NOT NULL,
  tanggal      DATE NOT NULL,
  jumlah_bayar DECIMAL(15,2) NOT NULL,
  keterangan   TEXT,
  user_id      BIGINT NOT NULL,
  created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (tagihan_id) REFERENCES spp_tagihan(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
```

### Tabel: tabungan_setoran
```sql
CREATE TABLE tabungan_setoran (
  id              BIGINT AUTO_INCREMENT PRIMARY KEY,
  siswa_id        BIGINT NOT NULL,
  tahun_ajaran_id BIGINT NOT NULL,
  tanggal         DATE NOT NULL,
  jumlah_setor    DECIMAL(15,2) NOT NULL,
  persen_potong   DECIMAL(5,2) NOT NULL,   -- snapshot % saat transaksi
  jumlah_potong   DECIMAL(15,2) NOT NULL,  -- hitung otomatis
  jumlah_bersih   DECIMAL(15,2) NOT NULL,  -- masuk saldo siswa
  user_id         BIGINT NOT NULL,
  created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (siswa_id) REFERENCES siswa(id),
  FOREIGN KEY (tahun_ajaran_id) REFERENCES tahun_ajaran(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
```

### Tabel: tabungan_potongan_akhir
```sql
-- Potongan tunggakan SPP di akhir tahun ajaran
CREATE TABLE tabungan_potongan_akhir (
  id              BIGINT AUTO_INCREMENT PRIMARY KEY,
  siswa_id        BIGINT NOT NULL,
  tahun_ajaran_id BIGINT NOT NULL,
  tagihan_id      BIGINT NOT NULL,
  jumlah_potong   DECIMAL(15,2) NOT NULL,
  keterangan      TEXT,
  tanggal_proses  DATE NOT NULL,
  user_id         BIGINT NOT NULL,
  created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (siswa_id) REFERENCES siswa(id),
  FOREIGN KEY (tahun_ajaran_id) REFERENCES tahun_ajaran(id),
  FOREIGN KEY (tagihan_id) REFERENCES spp_tagihan(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);
```

### Tabel: tabungan_saldo
```sql
CREATE TABLE tabungan_saldo (
  id                  BIGINT AUTO_INCREMENT PRIMARY KEY,
  siswa_id            BIGINT NOT NULL,
  tahun_ajaran_id     BIGINT NOT NULL,
  total_setor         DECIMAL(15,2) DEFAULT 0,
  total_potong_rutin  DECIMAL(15,2) DEFAULT 0,
  total_potong_akhir  DECIMAL(15,2) DEFAULT 0,
  saldo_akhir         DECIMAL(15,2) DEFAULT 0,
  status              ENUM('aktif', 'selesai') DEFAULT 'aktif',
  UNIQUE KEY uq_siswa_tahun (siswa_id, tahun_ajaran_id),
  FOREIGN KEY (siswa_id) REFERENCES siswa(id),
  FOREIGN KEY (tahun_ajaran_id) REFERENCES tahun_ajaran(id)
);
```

### Tabel: setting
```sql
CREATE TABLE setting (
  id         BIGINT AUTO_INCREMENT PRIMARY KEY,
  kunci      VARCHAR(50) NOT NULL UNIQUE,
  nilai      VARCHAR(255) NOT NULL,
  keterangan TEXT,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  updated_by BIGINT,
  FOREIGN KEY (updated_by) REFERENCES users(id)
);

-- Seed data awal
INSERT INTO setting (kunci, nilai, keterangan) VALUES
('persen_potongan_tabungan', '5', 'Persen potongan tabungan siswa untuk sekolah'),
('nama_sekolah', 'SD ...', 'Nama sekolah untuk header laporan');
```

---

## 5. Modul & Fitur

### 5.1 Autentikasi
- Login email + password → JWT disimpan di httpOnly cookie
- Middleware cek role setiap request
- Logout (hapus cookie)

### 5.2 Master Data (Admin only)
- CRUD Tahun Ajaran (set aktif/nonaktif)
- CRUD Kelas (terikat tahun ajaran)
- CRUD Siswa (NIS, nama, kelas)
- CRUD Jenis Pemasukan (bebas tambah kapan saja)
- CRUD Kategori Pengeluaran + Sub Kategori

### 5.3 Kas Sekolah (Admin, Bendahara)
- Input pemasukan → pilih jenis dari master
- Input pengeluaran → pilih kategori + sub kategori
- Daftar transaksi + filter tanggal/jenis
- Saldo kas real-time

### 5.4 SPP (Admin, Bendahara)
- Generate tagihan SPP bulk awal tahun
- Input pembayaran (bisa cicil)
- Status: lunas / belum / cicil
- Rekap tunggakan per kelas

### 5.5 Tabungan Siswa (Admin, Bendahara)
- Input setoran per siswa (potongan % otomatis dari setting)
- Input setoran bulk per kelas
- Riwayat + saldo per siswa

### 5.6 Tutup Tahun Ajaran (Admin only)
- Preview: saldo vs tunggakan per siswa
- Proses potong otomatis
- Siswa saldo kurang → flagged manual
- Finalisasi: cairkan atau carry over

### 5.7 Laporan (Semua role)
- Laporan kas bulanan / tahunan
- Pengeluaran per kategori
- Tabungan per siswa / kelas
- Rekap SPP per kelas / bulan
- Export PDF & Excel

### 5.8 Setting (Admin only)
- Ubah persen potongan tabungan
- Nama sekolah & info header laporan
- Manajemen user (tambah, edit role, nonaktifkan)

---

## 6. Routing Plan

```
# Auth
POST   /login
POST   /logout

# Dashboard
GET    /dashboard

# Master Data
GET    /master/siswa
POST   /master/siswa
PUT    /master/siswa/:id
DELETE /master/siswa/:id

GET    /master/kelas
POST   /master/kelas

GET    /master/jenis-pemasukan
POST   /master/jenis-pemasukan
PUT    /master/jenis-pemasukan/:id

GET    /master/kategori
POST   /master/kategori
GET    /master/kategori/:id/sub
POST   /master/kategori/:id/sub

# Kas
GET    /kas/pemasukan
POST   /kas/pemasukan
DELETE /kas/pemasukan/:id
GET    /kas/pengeluaran
POST   /kas/pengeluaran
DELETE /kas/pengeluaran/:id
GET    /kas/saldo

# SPP
GET    /spp/tagihan
POST   /spp/tagihan/generate
GET    /spp/tagihan/:id
POST   /spp/pembayaran
GET    /spp/tunggakan

# Tabungan
GET    /tabungan
POST   /tabungan/setor
POST   /tabungan/setor-bulk
GET    /tabungan/siswa/:id
GET    /tabungan/saldo

# Tutup Tahun
GET    /tutup-tahun/preview
POST   /tutup-tahun/proses

# Laporan
GET    /laporan/kas
GET    /laporan/pengeluaran
GET    /laporan/tabungan
GET    /laporan/spp
GET    /laporan/kas/export
GET    /laporan/tabungan/export

# Setting & User
GET    /setting
POST   /setting
GET    /setting/users
POST   /setting/users
PUT    /setting/users/:id
```

---

## 7. Role & Akses

| Modul | Admin | Bendahara | Kepala Sekolah |
|---|---|---|---|
| Master Data | Full | - | - |
| Kas Sekolah | Full | Full | Lihat |
| SPP | Full | Full | Lihat |
| Tabungan | Full | Full | Lihat |
| Tutup Tahun | Full | - | - |
| Laporan | Full | Full | Full |
| Setting | Full | - | - |
| User Management | Full | - | - |

---

## 8. Logika Bisnis Penting

### Perhitungan Setoran Tabungan
```
jumlah_setor  = input dari bendahara
persen_potong = ambil dari tabel setting (di-snapshot saat transaksi)
jumlah_potong = jumlah_setor × (persen_potong / 100)
jumlah_bersih = jumlah_setor - jumlah_potong

→ jumlah_potong masuk kas sekolah (jenis pemasukan "Potongan Tabungan")
→ jumlah_bersih masuk saldo tabungan siswa
```

### Proses Tutup Tahun
```
1. Ambil semua siswa aktif tahun ajaran tersebut
2. Tiap siswa:
   a. Hitung total tunggakan SPP (status != 'lunas')
   b. Cek saldo tabungan bersih
   c. Saldo >= tunggakan → potong, saldo berkurang
   d. Saldo < tunggakan → potong semua saldo, sisanya flagged
3. Catat di tabungan_potongan_akhir
4. Update tabungan_saldo.status = 'selesai'
5. Saldo akhir: cairkan ke orang tua ATAU carry over ke tahun baru
```

### Perubahan Persen Potongan
```
- Hanya berlaku untuk transaksi baru
- Transaksi lama tetap pakai persen lama (tersimpan di kolom persen_potong)
- Tidak ada recalculate transaksi lama
```

---

## 9. Urutan Pengerjaan

```
Phase 1 – Fondasi
  [ ] Setup project Go + Gin
  [ ] Koneksi MySQL + GORM
  [ ] Autentikasi JWT + middleware role
  [ ] Layout template HTML + sidebar

Phase 2 – Master Data
  [ ] CRUD Tahun Ajaran
  [ ] CRUD Kelas
  [ ] CRUD Siswa
  [ ] CRUD Jenis Pemasukan
  [ ] CRUD Kategori & Sub Kategori

Phase 3 – Transaksi Utama
  [ ] Kas Pemasukan
  [ ] Kas Pengeluaran
  [ ] Dashboard saldo kas

Phase 4 – SPP
  [ ] Generate tagihan SPP
  [ ] Pencatatan pembayaran
  [ ] Rekap & status

Phase 5 – Tabungan
  [ ] Setoran harian
  [ ] Setoran bulk per kelas
  [ ] Riwayat & saldo

Phase 6 – Tutup Tahun
  [ ] Preview proses
  [ ] Eksekusi pemotongan
  [ ] Finalisasi saldo

Phase 7 – Laporan & Export
  [ ] Laporan kas
  [ ] Laporan kategori pengeluaran
  [ ] Laporan tabungan
  [ ] Export PDF & Excel

Phase 8 – Polish
  [ ] Setting & user management
  [ ] Validasi input lengkap
  [ ] Testing
```

---

## 10. Setup & Dependency Go

### Inisialisasi project
```bash
mkdir school-finance && cd school-finance
go mod init school-finance
go get github.com/gin-gonic/gin
go get github.com/golang-jwt/jwt/v5
go get gorm.io/gorm
go get gorm.io/driver/mysql
go get github.com/joho/godotenv
go get github.com/signintech/gopdf
go get github.com/xuri/excelize/v2
```

### Dependency utama
```
github.com/gin-gonic/gin         # HTTP framework
gorm.io/gorm                     # ORM
gorm.io/driver/mysql             # MySQL driver
github.com/golang-jwt/jwt/v5    # JWT auth
github.com/joho/godotenv         # .env loader
github.com/signintech/gopdf      # Export PDF
github.com/xuri/excelize/v2      # Export Excel
```

### Tools development
```
Air (live reload)  : go install github.com/cosmtrek/air@latest
VS Code extension  : golang.go (Go official)
DB client          : TablePlus atau DBeaver
API testing        : Thunder Client (VS Code extension)
```

### Contoh file .env
```
DB_HOST=localhost
DB_PORT=3306
DB_NAME=school_finance
DB_USER=root
DB_PASS=yourpassword
JWT_SECRET=your-secret-key-ganti-ini
APP_PORT=8080
```
