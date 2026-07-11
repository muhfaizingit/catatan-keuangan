// Skrip global aplikasi. Saat ini minimal; akan berkembang seiring fitur.
// HTMX menangani sebagian besar interaktivitas.

// Tutup modal: kosongkan #modal-root.
function closeModal() {
  const root = document.getElementById('modal-root');
  if (root) root.innerHTML = '';
}

// Toast: dipicu lewat header HX-Trigger {"toast":{"type":"error|success","msg":"..."}}.
document.body.addEventListener('toast', function (e) {
  showToast(e.detail.msg, e.detail.type);
});

function showToast(msg, type) {
  const root = document.getElementById('toast-root');
  if (!root) return;
  const colors = {
    success: 'bg-green-600',
    error: 'bg-red-600',
  };
  const el = document.createElement('div');
  el.className = 'rounded-lg px-4 py-2.5 text-sm text-white shadow-lg ' + (colors[type] || 'bg-slate-800');
  el.textContent = msg;
  root.appendChild(el);
  setTimeout(function () { el.remove(); }, 3500);
}

// Format input rupiah (class "rupiah-input") dengan pemisah ribuan saat mengetik.
// Server tetap menerima nilai berformat lalu mengambil digitnya saja.
function formatRupiahValue(v) {
  const digits = (v || '').replace(/\D/g, '');
  if (!digits) return '';
  return digits.replace(/\B(?=(\d{3})+(?!\d))/g, '.');
}
document.addEventListener('input', function (e) {
  if (e.target && e.target.classList && e.target.classList.contains('rupiah-input')) {
    const pos = e.target.value.length - e.target.selectionStart;
    e.target.value = formatRupiahValue(e.target.value);
    // Pertahankan posisi kursor relatif terhadap akhir.
    const newPos = Math.max(0, e.target.value.length - pos);
    e.target.setSelectionRange(newPos, newPos);
  }
  // Pratinjau tabungan: setoran masuk penuh ke saldo; potongan hanya perkiraan.
  if (e.target && e.target.classList && e.target.classList.contains('tabungan-setor')) {
    const pct = parseFloat(e.target.dataset.persen || '0');
    const val = parseInt((e.target.value || '').replace(/\D/g, ''), 10) || 0;
    const potong = Math.round((val * pct) / 100);
    const sv = document.getElementById('preview-saldo');
    const pv = document.getElementById('preview-potong');
    if (sv) sv.textContent = 'Rp ' + (formatRupiahValue(String(val)) || '0');
    if (pv) pv.textContent = 'Rp ' + (formatRupiahValue(String(potong)) || '0');
  }
});

// Terapkan nominal default ke semua input nominal siswa (form terbitkan tagihan).
function applyTagihanNominal(el) {
  const val = formatRupiahValue(el.value || '');
  document.querySelectorAll('.tagihan-nominal').forEach(function (i) {
    i.value = val;
  });
}

// Filter opsi sub-kategori sesuai kategori terpilih (form pengeluaran).
function filterSub(katSelect) {
  const sub = document.getElementById('sub-select');
  if (!sub) return;
  const kat = katSelect.value;
  sub.value = '';
  Array.from(sub.options).forEach(function (opt) {
    if (!opt.dataset.kat) return; // opsi "Tanpa sub"
    opt.hidden = opt.dataset.kat !== kat;
  });
}

// Tutup modal dengan tombol Escape.
document.addEventListener('keydown', function (e) {
  if (e.key === 'Escape') closeModal();
});

// Setelah HTMX swap, fokuskan input pertama di dalam modal (entri cepat).
document.body.addEventListener('htmx:afterSwap', function (e) {
  if (e.target && e.target.id === 'modal-root') {
    const first = e.target.querySelector('input, select, textarea');
    if (first) first.focus();
  }
});

// Tutup sidebar otomatis setelah navigasi di layar kecil.
document.addEventListener('click', function (e) {
  const link = e.target.closest('#sidebar a');
  if (link && window.innerWidth < 1024) {
    document.getElementById('sidebar')?.classList.add('-translate-x-full');
    document.getElementById('sidebar-backdrop')?.classList.add('hidden');
  }
});
