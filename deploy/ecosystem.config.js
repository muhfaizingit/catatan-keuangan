// PM2 ecosystem config untuk Keuangan Sekolah.
// Jalankan di folder /var/www/tkaba/keuangan:
//   pm2 start ecosystem.config.js
//   pm2 save            # persist (biar auto-start saat boot via `pm2 startup`)
//
// cwd diset ke folder aplikasi supaya godotenv.Load() menemukan .env
// dan Go membaca templates/** + ./static dengan benar.

module.exports = {
  apps: [
    {
      name: "keuangan",
      cwd: "/var/www/tkaba/keuangan",
      script: "/var/www/tkaba/keuangan/keuangan",
      autorestart: true,
      watch: false,
      max_restarts: 10,
      restart_delay: 3000,
      env_production: {
        APP_ENV: "production",
      },
    },
  ],
};
