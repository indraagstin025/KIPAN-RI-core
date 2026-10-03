T  E  C  H  N  I  C  A  L     D  E  S  I  G  N     D  O  C  U  M  E  N  T 

# **Dokumen Desain Teknis SIM KIPAN** 

Sistem Informasi Manajemen Keanggotaan Pemuda Anti Narkoba 

Versi Dokumen : 1.1 Basis Spesifikasi : URD SIM KIPAN v2.0 Ruang Lingkup : Keanggotaan, Rekrutmen, Verifikasi, SK, Demisioner Status : Final - Seluruh Konfirmasi Terjawab Tanggal : Oktober 2026 

SIM KIPAN - Kader Inti Pemuda Anti Narkoba Oktober 2026 

## **DAFTAR ISI** 

|**1. RINGKASAN EKSEKUTIF...............................................................................................1**<br>**2. RUANG LINGKUP, REFERENSI, DAN KEPUTUSAN DESAIN................................2**|
|---|
|2.1 Tujuan dan Ruang Lingkup Dokumen...........................................................................2|
|2.2 Dokumen Referensi........................................................................................................3|
|2.3 Log Keputusan Desain (Decision Log)..........................................................................3|
|**3. ARSITEKTUR SISTEM DAN TEKNOLOGI..................................................................6**|
|3.1 Arsitektur Aplikasi.........................................................................................................6|
|3.2 Tumpukan Teknologi.....................................................................................................7|
|3.3 Peran Pengguna, RBAC, dan Isolasi Wilayah...............................................................7|
|**4. DESAIN BASIS DATA........................................................................................................9**|
|4.1 Diagram Relasi Entitas...................................................................................................9|
|4.2 Spesifikasi Tabel..........................................................................................................10|
|4.2.1 Domain Wilayah dan Referensi..........................................................................10|
|4.2.2 Domain Rekrutmen (pendaftaran dan berkas).....................................................11|
|4.2.3 Domain Keanggotaan (anggota dan counter NIA)..............................................13|
|4.2.4 Domain SK dan Kepengurusan...........................................................................14|
|4.2.5 Domain Akses dan Audit....................................................................................17|
|4.3 Generator Nomor Induk Anggota (NIA)......................................................................18|
|**5. LOGIKA BISNIS INTI......................................................................................................19**|
|5.1 Mesin Status Pendaftaran dan Verifikasi.....................................................................21|
|5.2 Pipeline Penerbitan NIA dan e-KTA............................................................................22|
|5.3 Siklus Hidup SK dan Single Active SK Rule...............................................................23|
|5.4 Deteksi Kedaluwarsa Dinamis (Dynamic Demisioner Check)....................................27|
|5.5 Algoritma Tiga Kolom Riwayat...................................................................................27|
|5.6 Pergantian Antar Waktu (PAW) dan Mutasi Jabatan Aktif.........................................28|
|**6. SPESIFIKASI API.............................................................................................................29**|



|6.1 Konvensi Umum...........................................................................................................29|
|---|
|6.2 API Autentikasi............................................................................................................29|
|6.3 API Publik: Pendaftaran dan Pelacakan Status............................................................30|
|6.4 API Verifikasi dan Putusan Kader...............................................................................30|
|6.5 API Anggota dan e-KTA..............................................................................................31|
|6.6 API Surat Keputusan dan Personalia............................................................................31|
|6.7 API Pengurus, PAW, dan Mutasi.................................................................................32|
|6.8 API Pendukung.............................................................................................................32|
|6.9 Contoh Muatan Dua Endpoint Kunci...........................................................................33|
|**7. KEAMANAN DATA DAN KEPATUHAN UU PDP......................................................34**|
|7.1 Perlindungan NIK: Enkripsi Dua Arah AES-256........................................................34|
|7.2 Jejak Audit Komprehensif............................................................................................34|
|7.3 Keamanan Berkas dan Validasi Unggahan..................................................................35|
|7.4 Keaslian e-KTA dan Tanda Tangan Statis...................................................................36|
|**8. REKOMENDASI IMPLEMENTASI DAN LANGKAH LANJUT..............................36**|
|8.1 Urutan Implementasi Bertahap.....................................................................................36|
|8.2 Risiko Implementasi dan Mitigasi................................................................................37|
|8.3 Penyelesaian Butir Konfirmasi.....................................................................................38|



_Catatan: Daftar isi ini dibuat dengan field code. Setelah dokumen disunting, klik kanan pada daftar isi lalu pilih Update Field untuk memperbarui nomor halaman._ 

Dokumen Desain Teknis SIM KIPAN - v1.1 

## **1. RINGKASAN EKSEKUTIF** 

Kader Inti Pemuda Anti Narkoba (KIPAN) adalah wadah pembinaan pemuda di bawah Kementerian Pemuda dan Olahraga Republik Indonesia yang tersebar pada tiga tingkat kepengurusan, yaitu Dewan Pengurus Nasional (DPN), Dewan Pengurus Daerah tingkat Provinsi (DPD), dan Dewan Pengurus Cabang tingkat Kabupaten/Kota (DPC). Dengan target pengelolaan ribuan kader secara tersebar, pengelolaan keanggotaan yang masih manual berisiko menimbulkan duplikasi data, keterlambatan verifikasi, serta hilangnya jejak histori kepengurusan. Atas dasar itu dikembangkan Sistem Informasi Manajemen Keanggotaan Pemuda Anti Narkoba (SIM KIPAN) sebagaimana dituangkan dalam dokumen User Requirements Document (URD) SIM KIPAN versi 2.0. 

Dokumen Desain Teknis (Technical Design Document/TDD) ini merupakan kelanjutan formal dari URD tersebut dan menjadi acuan tunggal tim pengembang pada fase pembangunan. Seluruh kebutuhan fungsional pada URD diterjemahkan ke dalam rancangan yang siap dieksekusi, meliputi: arsitektur aplikasi lima lapis, model basis data relasional dengan sebelas tabel inti beserta spesifikasi kolomnya, spesifikasi hampir empat puluh endpoint API, algoritma logika bisnis (mesin status pendaftaran, siklus Surat Keputusan, aturan Single Active SK, deteksi demisioner dinamis, serta algoritma tiga kolom riwayat), dan rancangan keamanan data yang patuh pada Undang-Undang Perlindungan Data Pribadi. Seluruh rancangan dipusatkan pada satu fokus manajerial: pendataan kader sejak pendaftaran, verifikasi, dan pengangkatan menjadi pengurus, hingga dinyatakan demisioner dengan riwayat kepengurusan yang utuh. 

Sejumlah keputusan desain telah dikonfirmasi langsung oleh pemilik produk, baik pada sesi klarifikasi awal maupun sesi konfirmasi final, dan mengikat seluruh rancangan. Keputusan yang paling berdampak adalah perubahan format Nomor Induk Anggota (NIA) menjadi **KIPAN-IND-[4 digit awal NIK]-[nomor urut 6 digit per kabupaten/kota]** dengan contoh KIPAN-IND-3217-000001, yang menggantikan format pada URD Bab 4.2. Keputusan lain yang turut mengunci meliputi: verifikasi pendaftaran kader cukup satu tahap oleh Admin Kabupaten/Kota wilayah pendaftar; rantai persetujuan SK yang berbeda per tingkat kepengurusan (SK DPC direview provinsi lalu disahkan nasional, SK DPD disahkan nasional saja, SK DPN langsung efektif); empat peran admin (Super Admin, Admin Nasional, Admin Provinsi, Admin Kabupaten/Kota) dengan hak akses dan wewenang masing-masing; larangan jabatan ganda bagi pengurus; mekanisme autentikasi ganda (admin memakai email dan kata sandi; anggota memakai NIK dan tanggal lahir); ketersediaan mutasi jabatan aktif dengan penutupan otomatis jabatan lama; keluaran e-KTA dalam dua format (PNG dan PDF) dengan citra tanda tangan statis Ketua Umum; pengisian master data wilayah secara nasional; serta penyimpanan permanen data pendaftar yang ditolak. 

1 

Dokumen Desain Teknis SIM KIPAN - v1.1 

Sistem direkomendasikan dibangun di atas tumpukan Next.js dengan basis data PostgreSQL dan Prisma ORM, diimplementasikan bertahap dalam empat fase dengan total durasi indikatif sepuluh minggu. Rincian urutan implementasi, analisis risiko, serta penyelesaian seluruh butir konfirmasi pemilik produk disajikan pada Bab 8. Dokumen ini ditujukan bagi pengembang, administrator basis data, penguji kualitas, dan pemilik produk sebagai rujukan teknis bersama. 

## **2. RUANG LINGKUP, REFERENSI, DAN KEPUTUSAN DESAIN** 

### **2.1 Tujuan dan Ruang Lingkup Dokumen** 

Dokumen ini bertujuan menerjemahkan seluruh kebutuhan fungsional URD SIM KIPAN v2.0 ke dalam rancangan teknis yang lengkap, tidak ambigu, dan dapat langsung dieksekusi oleh tim pengembang tanpa perlu menafsirkan ulang kebutuhan. Setiap keputusan pada dokumen ini bersifat mengikat bagi implementasi; apabila terdapat perbedaan dengan URD, maka keputusan pada log keputusan desain (Subbab 2.3) yang berlaku. Dokumen juga menjadi dasar penyusunan rencana pengujian karena seluruh status, transisi, dan efek samping bisnis telah didefinisikan secara eksplisit. 

Ruang lingkup pembangunan mencakup keseluruhan enam modul inti keanggotaan beserta kelengkapan sisi publik yang dipadankan secara minimal, yaitu: 

- Modul 1 - **Pendaftaran kader dan rekrutmen mandiri** : formulir empat bagian, unggah berkas, nomor registrasi, dan pelacakan status publik. 

- Modul 2 - **Verifikasi dan penerbitan NIA** : verifikasi satu tahap oleh Admin Kabupaten/Kota wilayah pendaftar hingga status kader resmi, generator NIA, dan e-KTA ber-QR code. 

- Modul 3 - **Standar data anggota dan aturan tiga kolom riwayat** : tampilan tabel anggota dengan kolom NAMA, PEKERJAAN, dan RIWAYAT. 

- Modul 4 - **Pengusulan Surat Keputusan pengurus** : penyusunan draf, personalia dari data kader resmi, dan rantai persetujuan yang berbeda per tingkat kepengurusan. 

- Modul 5 - **Tata kelola pergantian pengurus dan status demisioner** : Single Active SK Rule, deteksi kedaluwarsa dinamis, PAW, dan mutasi aktif. 

- Modul 6 - **Aturan tampilan dan isolasi wilayah** : default view per peran, pembatasan akses antar wilayah, dan filter Ketua Umum pada halaman publik. 

- **Sisi publik minimal** : halaman pendaftaran, halaman cek status (NIK + tanggal lahir), dan halaman validasi e-KTA (/validasi-kta/[nia]). 

2 

Dokumen Desain Teknis SIM KIPAN - v1.1 

Di luar lingkup (out of scope) pada fase ini: modul berita dan galeri, landing page etalase lengkap (testimoni, sambutan tokoh), integrasi pembayaran, aplikasi mobile native, notifikasi WhatsApp (ditunda hingga gateway resmi ditetapkan organisasi), serta manajemen kegiatan organisasi lainnya. Modul tersebut ditunda ke fase pengembangan berikutnya agar fokus kualitas pada inti keanggotaan tetap terjaga. Kode program lama hanya dijadikan referensi logika bisnis; seluruh basis kode ditulis ulang bersih di atas tumpukan teknologi baru. 

### **2.2 Dokumen Referensi** 

Dokumen ini disusun dengan merujuk pada sumber berikut: 

**Tabel 1. Dokumen dan sumber rujukan penyusunan desain teknis** 

|**No**|**Referensi**|**Peran dalam Desain**|
|---|---|---|
|1|URD SIM KIPAN v2.0 (Oktober<br>2026) - Fokus Khusus Modul<br>Pendataan Keanggotaan,<br>Rekrutmen Kader, Verifikasi,<br>Pengusulan SK, serta Tata Kelola<br>Pengurus dan Demisioner|Kebutuhan fungsional utama; dasar<br>seluruh mesin status dan aturan<br>bisnis|
|2|Hasil sesi klarifikasi dan<br>konfirmasi final pemilik produk<br>(terlampir dalam log keputusan<br>desain)|Mengunci tujuh belas keputusan<br>desain yang mengubah/melengkapi<br>URD|
|3|Undang-Undang Nomor 27 Tahun<br>2022 tentang Pelindungan Data<br>Pribadi (UU PDP)|Kerangka kepatuhan pengamanan<br>NIK, berkas, dan audit|
|4|Data kode dan nama wilayah<br>administrasi Kemendagri yang<br>berlaku (38 provinsi, sekitar 514<br>kabupaten/kota)|Master data wilayah; sumber kode<br>2 dan 4 digit yang dipakai NIK dan<br>NIA|



### **2.3 Log Keputusan Desain (Decision Log)** 

Tujuh belas keputusan berikut telah dikonfirmasi oleh pemilik produk - delapan dari sesi klarifikasi awal dan sembilan dari sesi konfirmasi final - dan **berlaku menggantikan ketentuan URD apabila terdapat perbedaan** . Setiap keputusan dicatat beserta konsekuensi teknisnya agar mudah dilacak saat pengujian maupun audit desain. Keputusan paling krusial adalah nomor satu mengenai format NIA karena berdampak pada skema basis data, algoritma penomoran, dan tampilan kartu; keputusan nomor delapan hingga sebelas menata ulang alur verifikasi kader dan rantai persetujuan SK sehingga mengikat seluruh mesin status pada Bab 5. 

3 

Dokumen Desain Teknis SIM KIPAN - v1.1 

**Tabel 2. Log keputusan desain yang dikonfirmasi pemilik produk** 

|**No**|**Topik**|**Keputusan**|**Konsekuensi Desain**|
|---|---|---|---|
|1|Format NIA|KIPAN-IND-[kode<br>wilayah 4 digit dari 4 digit<br>awal NIK pendaftar]-[urut<br>6 digit]; contoh: KIPAN-<br>IND-3217-000001|Menggantikan format URD<br>Bab 4.2 (KIPAN-<br>JBR-3204-2026-00012);<br>tabel niaCounter per<br>kabupaten/kota; tanpa<br>komponen tahun|
|2|Autentikasi|Admin: email + kata sandi;<br>Anggota resmi: NIK +<br>tanggal lahir|Dua jalur penerbitan token<br>JWT dengan klaim peran<br>berbeda; konsisten dengan<br>fitur cek status publik|
|3|Mutasi pengurus|Tersedia perpindahan<br>jabatan/wilayah selama<br>masa bakti berjalan;<br>jabatan lama otomatis<br>Demisioner|Endpoint mutasi menutup<br>record lama dan membuka<br>record riwayat baru tanpa<br>menghapus histori|
|4|Format e-KTA|Dapat diunduh sebagai<br>PNG dan PDF|Generator kartu merender<br>dua keluaran dari satu<br>templat dengan QR identik|
|5|Master wilayah|Nasional lengkap: 38<br>provinsi + sekitar 514<br>kabupaten/kota|Skrip seed wilayah wajib<br>pada fase awal; relasi<br>provinsi-kabupaten dipakai<br>scoping RBAC|
|6|Lingkup publik|Inti saja: form pendaftaran<br>+ cek status + validasi<br>KTA|Modul berita/galeri dan<br>landing page etalase<br>ditunda ke fase berikutnya|
|7|Basis kode|Kode lama hanya referensi<br>logika; ditulis ulang bersih|Arsitektur baru Next.js<br>fullstack; tidak ada migrasi<br>basis kode lama|
|8|Alur verifikasi kader|Direvisi dari URD: satu<br>tahap saja oleh Admin<br>Kabupaten/Kota wilayah<br>pendaftar (periksa<br>kelengkapan lalu<br>setujui/tolak); tidak lagi<br>melibatkan admin provinsi|Mesin status Bab 5.1<br>disederhanakan menjadi<br>lima status; hanya admin<br>wilayah pendaftar yang<br>dapat memproses<br>antreannya|
|9|Rantai persetujuan SK|SK DPC: direview Admin<br>Provinsi lalu disahkan<br>Admin Nasional; SK DPD:<br>disahkan Admin Nasional<br>saja; SK DPN: langsung<br>efektif tanpa persetujuan|Tiga varian rantai pada<br>Bab 5.3; kolom<br>reviewProvinsiOleh pada<br>tabel suratKeputusan|
|10|Larangan jabatan ganda|Satu anggota hanya boleh<br>memiliki satu jabatan<br>pengurus aktif pada satu<br>waktu, lintas tingkat<br>sekalipun|Partial unique index<br>(anggotaId WHERE status<br>AKTIF) + validasi saat<br>personalia SK ditambah<br>dan saat pengesahan|



4 

Dokumen Desain Teknis SIM KIPAN - v1.1 

|**No**|**Topik**|**Keputusan**|**Konsekuensi Desain**|
|---|---|---|---|
|11|Isolasi wilayah admin|Admin hanya dapat<br>mengubah data wilayah<br>yurisdiksinya; melihat data<br>wilayah lain diperbolehkan<br>namun bersifat baca-saja<br>dan tercatat|Middleware scoping dua<br>mode (tulis vs baca-saja) +<br>audit<br>LIHAT_LINTAS_WILAY<br>AH|
|12|Lingkup nomor urut NIA|Konfirmasi final: per<br>kabupaten/kota; setiap<br>wilayah mulai dari 000001<br>dengan batas 999.999<br>kader|Struktur niaCounter tetap<br>satu baris per<br>kabupaten/kota;<br>penomoran antarwilayah<br>saling mandiri|
|13|Nomor SK|Diisi manual oleh admin<br>penyusun mengikuti<br>kebijakan sekretariat;<br>unggah lampiran berkas<br>SK wajib|Kolom nomorSK teks<br>bebas namun unik;<br>pengajuan tanpa lampiran<br>ditolak validasi|
|14|Master jabatan|Dikelola manual; admin<br>dapat menambahkan<br>sendiri jabatan yang belum<br>ada, termasuk menandai<br>isKetuaUmum|CRUD masterJabatan oleh<br>semua peran admin + audit<br>TAMBAH_JABATAN;<br>tanpa daftar tetap bawaan<br>sistem|
|15|Notifikasi WhatsApp|Ditunda: pemilihan<br>gateway belum dilakukan<br>pada fase ini|Pelacakan status<br>sepenuhnya via menu<br>publik (NIK + tanggal<br>lahir); hook notifikasi tetap<br>disiapkan pada arsitektur|
|16|Retensi pendaftar ditolak|Data pendaftar berstatus<br>DITOLAK tidak pernah<br>dihapus; disimpan<br>permanen sebagai arsip|Tanpa cron penghapusan;<br>akses arsip dibatasi dan<br>teraudit; pendaftar dapat<br>mendaftar ulang (indeks<br>unik nikHash hanya untuk<br>status non-DITOLAK)|
|17|Tanda tangan e-KTA|Citra tanda tangan statis<br>Ketua Umum DPN yang<br>telah tersedia; bukan tanda<br>tangan elektronik<br>tersertifikasi|Aset terproteksi pada<br>generator e-KTA; keaslian<br>kartu diverifikasi melalui<br>QR ke halaman publik|



**Perhatian format NIA:** karena kode wilayah diambil dari empat digit awal NIK pendaftar, dimungkinkan NIK luar domisili menghasilkan NIA dengan kode wilayah asal KTP. Perilaku ini sesuai keputusan pemilik produk; anomali tersebut tetap dilaporkan pada dashboard statistik agar dapat ditinjau organisasi (lihat Subbab 4.3). 

**Prinsip pemisahan Data Anggota dan Data Pengurus:** personalia SK selalu diambil dari data kader resmi (bukan data bebas), dan pengangkatan menjadi pengurus tidak pernah mengubah baris data anggota. Yang bertambah hanyalah label status kepengurusan (misalnya "Sedang menjabat: Pengurus Kabupaten/Kota") beserta entri riwayat yang memuat jabatan, wilayah, dan periode bakti, sehingga setelah demisioner tetap tertera kader tersebut pernah menjadi bagian dari kepengurusan pada periode tertentu. Rincian teknisnya diuraikan pada Subbab 5.3 dan 5.5. 

5 

Dokumen Desain Teknis SIM KIPAN - v1.1 

## **3. ARSITEKTUR SISTEM DAN TEKNOLOGI** 

### **3.1 Arsitektur Aplikasi** 

SIM KIPAN dibangun sebagai satu aplikasi fullstack Next.js yang memisahkan tanggung jawab ke dalam lima lapisan sebagaimana ditunjukkan pada Gambar 1. Pemilihan arsitektur monolitik termodular (bukan microservice) didasarkan pada skala organisasi, ukuran tim yang ramping, dan kebutuhan konsistensi transaksi basis data yang tinggi, khususnya pada alur pengesahan SK yang menyentuh banyak tabel sekaligus. Pemisahan lapisan tetap dijaga ketat melalui struktur direktori dan aturan impor sehingga migrasi ke layanan terpisah di masa depan tidak memerlukan penulisan ulang besar. 



<!-- Start of picture text -->
Arsitektur Teknis SIM KIPAN<br><!-- End of picture text -->

_Gambar 1. Arsitektur teknis lima lapis SIM KIPAN_ 

Lapisan presentasi menangani tiga kelompok pengguna dengan kebutuhan berbeda: pengunjung publik yang membutuhkan halaman cepat dan terindeks mesin pencari, admin yang membutuhkan dashboard padat interaksi, dan anggota yang membutuhkan akses profil 

6 

Dokumen Desain Teknis SIM KIPAN - v1.1 

serta e-KTA dari perangkat seluler. Lapisan API berupa Route Handlers dengan middleware scoping yang menegakkan isolasi wilayah pada setiap permintaan; tidak ada endpoint data yang dapat diakses tanpa melalui middleware ini. Lapisan layanan domain memuat seluruh logika bisnis (verifikasi, SK, riwayat, e-KTA, audit) sehingga aturan-aturan kritis teruji unit secara terpusat dan tidak tersebar di handler. Lapisan data terdiri atas PostgreSQL sebagai sumber kebenaran tunggal, object storage untuk berkas, serta worker cron harian yang mematerialisasi deteksi SK kedaluwarsa. 

### **3.2 Tumpukan Teknologi** 

Pemilihan teknologi berorientasi pada produktivitas tim, ketersediaan talenta di ekosistem Indonesia, dan kepatuhan terhadap ketentuan non-fungsional URD (responsif multi-perangkat, kompatibilitas peramban modern, dan keamanan data pribadi): 

**Tabel 3. Tumpukan teknologi rekomendasi beserta peruntukannya** 

|**Lapisan**|**Teknologi**|**Catatan Desain**|
|---|---|---|
|Aplikasi|Next.js 16 (App Router) +<br>TypeScript|SSR untuk halaman publik (SEO cek<br>status dan validasi KTA); API Route<br>Handlers terkolokasi|
|Basis data|PostgreSQL 16 + Prisma ORM|Transaksi ACID untuk NIA dan SK;<br>migrasi skema versi terkontrol|
|Autentikasi|jose (JWT) + bcrypt (cost 12)|Dua jalur login; klaim: peran,<br>provinsiId, kabupatenKotaId,<br>anggotaId|
|Validasi|Zod|Skema validasi dibagikan antara<br>formulir klien dan handler server|
|Berkas|Object storage privat / penyimpanan<br>lokal terproteksi|Akses berkas hanya melalui endpoint<br>terautentikasi dengan token tanda<br>tangan|
|Kartu e-KTA|qrcode + sharp + pdf-lib|Render PNG (rasio 85,6 x 53,98 mm)<br>dan PDF satu halaman dari satu<br>templat|
|Tugas terjadwal|node-cron (proses worker terpisah)|Deteksi kedaluwarsa SK harian +<br>laporan anomali NIA|
|UI|React 19 + Tailwind CSS 4|Responsif desktop/tablet/ponsel;<br>komponen tabel tiga kolom dan<br>modal pencarian kader|



### **3.3 Peran Pengguna, RBAC, dan Isolasi Wilayah** 

Model kendali akses berbasis peran (Role-Based Access Control/RBAC) menegakkan empat peran admin - **Super Admin, Admin Nasional, Admin Provinsi, dan Admin Kabupaten/Kota** - ditambah dua kelompok pengguna non-admin (pendaftar dan anggota resmi), dengan ruang lingkup wilayah yang melekat pada akun. Setiap peran admin memiliki hak akses dan wewenang berbeda: apa yang boleh dan tidak boleh dilakukan didefinisikan 

7 

Dokumen Desain Teknis SIM KIPAN - v1.1 

eksplisit pada matriks berikut. Prinsip utamanya: **setiap query data wajib difilter oleh klaim wilayah sesi** , bukan hanya disembunyikan di antarmuka. Middleware API membaca klaim JWT (peran, provinsiId, kabupatenKotaId) lalu menyuntikkan filter wilayah ke setiap kueri Prisma sehingga, sebagai contoh, admin Kabupaten Bandung Barat secara teknis tidak mungkin mengubah data pendaftar Kabupaten Bekasi meskipun memanipulasi parameter permintaan. 

**Tabel 4. Matriks peran, wewenang, dan larangan akses** 

|**Peran**|**Ruang Lingkup Data**|**Verifikasi**|**Wewenang SK**|**Isolasi / Larangan**|
|---|---|---|---|---|
|Super Admin|Seluruh Indonesia<br>(akun teknis tertinggi)|Bypass pemulihan<br>semua alur verifikasi|Semua operasi SK;<br>kelola akun admin<br>semua level, master<br>data, dan konfigurasi<br>sistem|Tanpa batas wilayah;<br>setiap aksi<br>bypass/pemulihan<br>wajib tercatat audit;<br>jumlah akun dijaga<br>minimal|
|Admin Nasional<br>(DPN)|Seluruh Indonesia|Bypass pemulihan<br>verifikasi kader|Mengesahkan SK<br>DPD dan SK DPC<br>(tahap final);<br>menginput SK DPN<br>langsung tanpa<br>persetujuan|Melihat semua<br>wilayah; menulis data<br>lintas wilayah hanya<br>melalui alur resmi<br>yang teraudit|
|Admin Provinsi<br>(DPD)|1 provinsi + seluruh<br>kab/kota di bawahnya|Tidak memverifikasi<br>kader (hanya pantau<br>antrean wilayahnya)|Review dan<br>menyetujui usulan SK<br>kab/kota (tahap 1);<br>menyusun dan<br>mengajukan SK<br>daerah (DPD)|Melihat wilayah lain<br>baca-saja; dilarang<br>mengubah data di luar<br>provinsinya|
|Admin<br>Kabupaten/Kota<br>(DPC)|1 kabupaten/kota<br>terdaftar|Memverifikasi dan<br>memutuskan<br>pendaftaran kader<br>wilayahnya (satu<br>tahap:<br>setujui/perbaikan/tola<br>k)|Menyusun dan<br>mengajukan SK<br>cabang (DPC)|Melihat wilayah lain<br>baca-saja; dilarang<br>mengubah data di luar<br>kab/kotanya|
|Anggota resmi|Profil dan e-KTA<br>miliknya|Tidak ada|Dapat diangkat<br>menjadi personalia SK|Tidak dapat melihat<br>data anggota lain|
|Pendaftar (calon<br>kader)|Data dirinya sendiri<br>(publik)|Tidak ada|Tidak ada|Hanya cek status via<br>NIK + tanggal lahir|



Mekanisme **baca-saja lintas wilayah** : untuk keperluan rujukan dan koordinasi antarwilayah, admin dapat membuka data wilayah di luar yurisdiksinya melalui sakelar eksplisit "lihat wilayah lain". Pada mode tersebut antarmuka menampilkan pita peringatan baca-saja, seluruh tombol aksi (verifikasi, sunting, hapus, persetujuan) disembunyikan, dan setiap pembukaan dicatat pada audit log dengan aksi LIHAT_LINTAS_WILAYAH. Default view tetap mengarah ke wilayah yurisdiksi: misalnya pendaftar baru di Kabupaten Bandung Barat hanya akan muncul pada antrean Admin Bandung Barat, dan hanya admin itulah yang dapat memvalidasi serta memverifikasi pendaftar tersebut. 

Default view menu Data Pengurus otomatis mengikuti peran: Admin Kabupaten/Kota langsung disajikan daftar pengurus cabangnya, Admin Provinsi disajikan pengurus 

8 

Dokumen Desain Teknis SIM KIPAN - v1.1 

daerahnya dengan opsi filter turun ke kab/kota di wilayahnya, sedangkan Admin Nasional dan Super Admin disajikan tingkat nasional dengan dropdown fleksibel seluruh provinsi dan kabupaten/kota. Pada sisi publik, komponen etalase pimpinan (misalnya daftar Ketua Umum) hanya menampilkan pengurus dengan jabatan **Ketua Umum** pada tingkat DPN, DPD, dan DPC; jabatan lain tidak dimunculkan untuk menjaga eksklusivitas profil pimpinan tertinggi. Penyaringan ini diimplementasikan pada kueri publik dengan kondisi masterJabatan.isKetuaUmum = true dan status pengurus efektif aktif. 

## **4. DESAIN BASIS DATA** 

### **4.1 Diagram Relasi Entitas** 

Basis data dirancang sebagai satu basis PostgreSQL dengan sebelas tabel inti yang terkelompok dalam empat domain fungsional: wilayah dan referensi, rekrutmen dan keanggotaan, SK dan kepengurusan, serta akses dan audit. Gambar 2 memperlihatkan relasi utama antarentitas beserta kardinalitasnya. Seluruh penamaan tabel dan kolom menggunakan camelCase berbahasa Indonesia agar selaras dengan istilah URD dan memudahkan komunikasi dengan pemilik produk; penamaan tersebut dipertahankan konsisten dari basis data hingga lapisan API. 



<!-- Start of picture text -->
Diagram Relasi Entitas (ERD) — Basis Data SIM KIPAN<br>1“ o a o a o ‘a o<br>ME Bon PRR tnceoraan bec<br>“ o “ o ra o<br><!-- End of picture text -->

_Gambar 2. Diagram relasi entitas (ERD) sebelas tabel inti SIM KIPAN_ 

9 

Dokumen Desain Teknis SIM KIPAN - v1.1 

Tiga prinsip relasional dijaga ketat pada desain ini. Pertama, **integritas referensial** seluruh relasi ditegakkan lewat foreign key dengan onDelete RESTRICT sehingga tidak ada data induk yang dapat dihapus ketika masih dirujuk. Kedua, **immutabilitas riwayat** : record pengurus tidak pernah diubah strukturnya saat pergantian jabatan; yang terjadi adalah penutupan record lama (status + tanggalBerakhir + alasanPerubahan) dan pembukaan record baru, sehingga histori tiga kolom riwayat selalu dapat direkonstruksi. Ketiga, **auditabilitas menyeluruh** : tabel auditLog bersifat append-only dan tidak pernah dihapus melalui antarmuka aplikasi. 

### **4.2 Spesifikasi Tabel** 

Spesifikasi berikut mencantumkan seluruh kolom beserta tipe, batasan, dan keterangannya. Tipe mengacu pada tipe Prisma/PostgreSQL; nilai bawaan (default) dan kolom createdAt/updatedAt dikelola otomatis oleh Prisma ( @default(now()) dan @updatedAt ) dan dicantumkan eksplisit agar tidak terlewat saat peninjauan. 

#### **4.2.1 Domain Wilayah dan Referensi** 

**Tabel 5. Spesifikasi tabel provinsi** 

|**Kolom**|**Tipe**|**Batasan**|**Keterangan**|
|---|---|---|---|
|`id`|Int|PK, autoincrement|Identitas unik provinsi|
|`kodeKemendagri`|Char(2)|UNIQUE, NOT NULL|Kode resmi Kemendagri;<br>contoh 32 = Jawa Barat|
|`nama`|Varchar(100)|NOT NULL|Nama resmi provinsi|
|`createdAt /`|DateTime|Otomatis|Jejak waktu pembuatan|
|`updatedAt`|||dan perubahan|



**Tabel 6. Spesifikasi tabel kabupatenKota** 

|**Kolom**|**Tipe**|**Batasan**|**Keterangan**|
|---|---|---|---|
|`id`|Int|PK, autoincrement|Identitas unik<br>kabupaten/kota|
|`provinsiId`|Int|FK ke provinsi.id, NOT<br>NULL|Induk provinsi; onDelete<br>RESTRICT|
|`kodeKemendagri`|Char(4)|UNIQUE, NOT NULL|Kode 4 digit Kemendagri<br>(2 digit provinsi + 2 digit<br>kab/kota), contoh 3217 =<br>Kab. Bandung; menjadi<br>kode wilayah NIA dan<br>pembanding 4 digit awal<br>NIK|
|`nama`|Varchar(120)|NOT NULL|Nama resmi<br>kabupaten/kota|
|`createdAt /`<br>`updatedAt`|DateTime|Otomatis|Jejak waktu|



10 

Dokumen Desain Teknis SIM KIPAN - v1.1 

#### **4.2.2 Domain Rekrutmen (pendaftaran dan berkas)** 

**Tabel 7. Spesifikasi tabel pendaftaran** 

|**Kolom**|**Tipe**|**Batasan**|**Keterangan**|
|---|---|---|---|
|`id`|Int|PK, autoincrement|Identitas baris pendaftaran|
|`nomorRegistrasi`|Varchar(20)|UNIQUE, NOT NULL|Format REG-YYYYMM-<br>XXXX; diterbitkan saat<br>submit|
|`nikTerenkripsi`|Text|NOT NULL|NIK terenkripsi AES-256-<br>GCM (base64); lihat Bab 7|
|`nikHash`|Char(64)|UNIQUE (parsial), NOT<br>NULL|SHA-256 atas NIK; kunci<br>deteksi duplikasi dan<br>pencarian tanpa dekripsi.<br>Indeks unik hanya<br>diberlakukan pada status<br>non-DITOLAK agar<br>pendaftar yang pernah<br>ditolak dapat mendaftar<br>ulang tanpa arsip lama<br>dihapus|
|`namaLengkap`|Varchar(150)|NOT NULL|Sesuai e-KTP, tanpa<br>singkatan|
|`tempatLahir`|Varchar(100)|NOT NULL|-|
|`tanggalLahir`|Date|NOT NULL|Basis validasi usia 16-30<br>tahun dan login anggota|
|`jenisKelamin`|Enum|NOT NULL|LAKI_LAKI /<br>PEREMPUAN|
|`agama`|Enum|NOT NULL|ISLAM /<br>KRISTEN_PROTESTAN /<br>KATOLIK / HINDU /<br>BUDDHA /<br>KHONGHUCU /<br>LAINNYA|
|`statusPernikahan`|Enum|NOT NULL|BELUM_MENIKAH /<br>MENIKAH|
|`provinsiId`|Int|FK provinsi.id, NOT<br>NULL|Domisili pendaftar|
|`kabupatenKotaId`|Int|FK kabupatenKota.id,<br>NOT NULL|Domisili pendaftar;<br>penentu admin verifikator<br>(hanya Admin Kab/Kota<br>setempat yang memproses)|
|`kecamatan`|Varchar(100)|NOT NULL|Teks deskriptif|
|`kelurahanDesa`|Varchar(100)|NOT NULL|Teks deskriptif|
|`alamatLengkap`|Varchar(255)|NOT NULL|Jalan/RT/RW|
|`kodePos`|Char(5)|NOT NULL|-|
|`pekerjaan`|Varchar(100)|NOT NULL|Dasar kolom<br>PEKERJAAN pada tabel<br>anggota; ditampilkan '-'<br>bila kosong|



11 

Dokumen Desain Teknis SIM KIPAN - v1.1 

|**Kolom**|**Tipe**|**Batasan**|**Keterangan**|
|---|---|---|---|
|`pendidikanTerakhir`|Varchar(50)|NOT NULL|SLTA/D3/S1/S2/dst.|
|`nomorWhatsApp`|Varchar(20)|NOT NULL|Kanal notifikasi status|
|`email`|Varchar(150)|NOT NULL|Korespondensi resmi|
|`motivasi`|Text|Opsional|Alasan menjadi kader|
|`paktaIntegritas`|Boolean|NOT NULL, default false|Wajib true saat submit|
|`status`|Enum|NOT NULL, default<br>DRAFT|DRAFT/DIKIRIM/<br>PERBAIKAN/<br>DISETUJUI/DITOLAK<br>(lihat Bab 5.1); DIKIRIM<br>tampil ke pendaftar sebagai<br>"Diterima/Dikirim"|
|`catatanRevisi`|Text|Opsional|Catatan verifikator saat<br>status PERBAIKAN|
|`alasanPenolakan`|Text|Opsional|Diisi saat status DITOLAK<br>final|
|`diverifikasiOleh`|Int|FK adminUser.id, opsional|Admin kab/kota wilayah<br>pendaftar yang memproses<br>verifikasi dan putusan|
|`tanggalDiajukan`|DateTime|Opsional|Waktu submit<br>pertama/terakhir|
|`tanggalVerifikasi`|DateTime|Opsional|Waktu putusan admin<br>kab/kota (setujui/tolak)|
|`tanggalDisetujui`|DateTime|Opsional|Waktu persetujuan final;<br>saat NIA diterbitkan|
|`anggotaId`|Int|FK anggota.id, opsional|Diisi setelah konversi<br>menjadi anggota|
|`createdAt /`<br>`updatedAt`|DateTime|Otomatis|Jejak waktu|



**Kebijakan retensi penolakan:** sesuai keputusan desain nomor 16, baris pendaftaran berstatus DITOLAK disimpan permanen dan tidak pernah dihapus otomatis. Arsip tersebut tetap dapat ditinjau oleh verifikator wilayah setempat serta Super Admin/Admin Nasional untuk keperluan audit, dan pendaftar bersangkutan tetap dapat mengajukan pendaftaran baru dengan NIK yang sama karena indeks unik nikHash hanya diberlakukan pada status non-DITOLAK. 

**Tabel 8. Spesifikasi tabel berkasPendaftaran** 

|**Kolom**|**Tipe**|**Batasan**|**Keterangan**|
|---|---|---|---|
|`id`|Int|PK, autoincrement|Identitas berkas|
|`pendaftaranId`|Int|FK pendaftaran.id, NOT|Pemilik berkas; onDelete|
|||NULL|CASCADE|



12 

Dokumen Desain Teknis SIM KIPAN - v1.1 

|**Kolom**|**Tipe**|**Batasan**|**Keterangan**|
|---|---|---|---|
|`jenis`|Enum|NOT NULL|PAS_FOTO/<br>FOTO_KTP/CV/<br>PAKTA_INTEGRITAS/<br>SURAT_BEBAS_NARK<br>OBA/<br>SERTIFIKAT_PELATIH<br>AN; PAS_FOTO dan<br>FOTO_KTP wajib ada<br>minimal satu versi<br>terakhir|
|`filePath`|Varchar(255)|NOT NULL|Lokasi penyimpanan<br>privat; nama acak UUID|
|`namaFileAsli`|Varchar(255)|NOT NULL|Nama unggahan asal utk<br>jejak|
|`mimeType`|Varchar(100)|NOT NULL|Whitelist: image/jpeg,<br>image/png,<br>application/pdf|
|`ukuranBytes`|Int|NOT NULL, maks<br>2.097.152|Batas 2 MB per berkas|
|`versi`|Int|NOT NULL, default 1|Bertambah setiap unggah<br>ulang saat PERBAIKAN;<br>versi lama tetap<br>tersimpan|
|`createdAt`|DateTime|Otomatis|Waktu unggah|



#### **4.2.3 Domain Keanggotaan (anggota dan counter NIA)** 

**Tabel 9. Spesifikasi tabel anggota** 

|**Kolom**|**Tipe**|**Batasan**|**Keterangan**|
|---|---|---|---|
|`id`|Int|PK, autoincrement|Identitas anggota|
|`nia`|Varchar(30)|UNIQUE, NOT NULL|Format KIPAN-IND-[kode<br>4 digit dari NIK]-[urut 6<br>digit]|
|`nikTerenkripsi /`<br>`nikHash`|Text / Char(64)|NOT NULL / UNIQUE|Duplikasi struktur<br>pendaftaran; dipertahankan<br>utk keamanan dan<br>pencarian|
|`pendaftaranId`|Int|FK pendaftaran.id,<br>UNIQUE, opsional|Jejak asal pendaftaran; null<br>hanya utk bypass Super<br>Admin/Admin Nasional|
|`namaLengkap`|Varchar(150)|NOT NULL|Kolom NAMA (tebal) +<br>NIA di bawahnya|
|`tempatLahir /`<br>`tanggalLahir`|Varchar / Date|NOT NULL|Dipakai login anggota<br>(NIK + tanggal lahir)|
|`jenisKelamin /`<br>`agama /`<br>`statusPernikahan`|Enum|NOT NULL|Sama dengan pendaftaran|



13 

Dokumen Desain Teknis SIM KIPAN - v1.1 

|**Kolom**|**Tipe**|**Batasan**|**Keterangan**|
|---|---|---|---|
|`provinsiId /`<br>`kabupatenKotaId`|Int / Int|FK, NOT NULL|Domisili; kunci scoping<br>dan statistik|
|`kecamatan /`<br>`kelurahanDesa /`<br>`alamatLengkap /`<br>`kodePos`|Varchar|NOT NULL|Detail alamat|
|`pekerjaan /`<br>`pendidikanTerakhir`|Varchar|NOT NULL|pekerjaan = kolom<br>PEKERJAAN|
|`nomorWhatsApp /`<br>`email / motivasi`|Varchar / Text|NOT NULL / opsional|Kontak aktif|
|`fotoPath`|Varchar(255)|NOT NULL|Pas foto resmi utk e-KTA|
|`statusAnggota`|Enum|NOT NULL, default<br>AKTIF|AKTIF / NONAKTIF /<br>DIBERHENTIKAN /<br>MENINGGAL|
|`tanggalDisetujui`|DateTime|NOT NULL|Tanggal terbit<br>keanggotaan; tercetak di e-<br>KTA|
|`createdAt /`<br>`updatedAt`|DateTime|Otomatis|Jejak waktu|



**Tabel 10. Spesifikasi tabel niaCounter** 

|**Kolom**|**Tipe**|**Batasan**|**Keterangan**|
|---|---|---|---|
|`kabupatenKotaId`|Int|PK sekaligus FK<br>kabupatenKota.id|Satu counter per<br>kabupaten/kota (kode<br>wilayah NIA)|
|`lastNumber`|Int|NOT NULL, default 0|Nomor urut terakhir yang<br>terpakai; dinaikkan dalam<br>transaksi terkunci (lihat<br>4.3)|



#### **4.2.4 Domain SK dan Kepengurusan** 

**Tabel 11. Spesifikasi tabel suratKeputusan** 

|**Kolom**|**Tipe**|**Batasan**|**Keterangan**|
|---|---|---|---|
|`id`|Int|PK, autoincrement|Identitas SK|
|`nomorSK`|Varchar(60)|UNIQUE, NOT NULL|Diisi manual oleh admin<br>penyusun (bebas format<br>sekretariat), contoh<br>SK-021/DPC-BBR/X/2026|
|`judulSK`|Varchar(255)|NOT NULL|Perihal pengangkatan|
|`level`|Enum|NOT NULL|NASIONAL / PROVINSI /<br>KABUPATEN; SK<br>NASIONAL langsung<br>DISETUJUI tanpa rantai<br>persetujuan|
|`provinsiId`|Int|FK, wajib bila level<br>PROVINSI|Wilayah DPD|



14 

Dokumen Desain Teknis SIM KIPAN - v1.1 

|**Kolom**|**Tipe**|**Batasan**|**Keterangan**|
|---|---|---|---|
|`kabupatenKotaId`|Int|FK, wajib bila level<br>KABUPATEN|Wilayah DPC|
|`tanggalTerbit`|Date|NOT NULL|Awal periode bakti|
|`tanggalBerakhir`|Date|NOT NULL, ><br>tanggalTerbit|Umumnya 3 tahun; dasar<br>deteksi demisioner dinamis|
|`lampiranPath`|Varchar(255)|NOT NULL|Berkas SK fisik<br>(PDF/scan); wajib<br>diunggah - pengajuan tanpa<br>lampiran ditolak validasi|
|`approvalStatus`|Enum|NOT NULL, default<br>DRAFT|DRAFT/<br>MENUNGGU_PROVINSI<br>/<br>MENUNGGU_NASIONA<br>L/DISETUJUI/DITOLAK;<br>rantainya berbeda per<br>tingkat (lihat Bab 5.3)|
|`catatanPenolakan`|Text|Opsional|Diisi saat DITOLAK|
|`statusAktif`|Boolean|NOT NULL, default false|true hanya utk SK<br>DISETUJUI terkini per<br>wilayah (Single Active SK)|
|`diajukanOleh`|Int|FK adminUser.id, NOT<br>NULL|Penyusun usulan|
|`reviewProvinsiOleh`|Int|FK adminUser.id, opsional|Admin provinsi pereview<br>usulan SK kab/kota (tahap<br>1 dari dua)|
|`tanggalReviewProvi`<br>`nsi`|DateTime|Opsional|Waktu lolos review<br>provinsi; SK kab/kota<br>beralih ke<br>MENUNGGU_NASIONA<br>L|
|`disetujuiOleh`|Int|FK adminUser.id, opsional|Pengesah akhir (Admin<br>Nasional; utk SK DPN =<br>penyusun sendiri)|
|`tanggalPengesahan`|DateTime|Opsional|Waktu DISETUJUI;<br>pemicu otomatisasi<br>demisioner|
|`createdAt /`<br>`updatedAt`|DateTime|Otomatis|Jejak waktu|



**Tabel 12. Spesifikasi tabel pengurus** 

|**Kolom**|**Tipe**|**Batasan**|**Keterangan**|
|---|---|---|---|
|`id`|Int|PK, autoincrement|Identitas record<br>kepengurusan|
|`anggotaId`|Int|FK anggota.id, NOT<br>NULL|Kader yang menjabat;<br>wajib kader resmi pada<br>cakupan wilayah sesuai<br>tingkat SK (Bab 5.3);<br>onDelete RESTRICT|



15 

Dokumen Desain Teknis SIM KIPAN - v1.1 

|**Kolom**|**Tipe**|**Batasan**|**Keterangan**|
|---|---|---|---|
|`suratKeputusanId`|Int|FK suratKeputusan.id,<br>NOT NULL|SK pengangkatan|
|`jabatanId`|Int|FK masterJabatan.id, NOT<br>NULL|Jabatan pada SK tsb.|
|`status`|Enum|NOT NULL, default<br>AKTIF|AKTIF/DEMISIONER/<br>DIBERHENTIKAN/<br>MENGUNDURKAN_DIR<br>I/MENINGGAL_DUNIA|
|`tanggalMulai`|Date|NOT NULL|Mulai menjabat (awalnya =<br>tanggalTerbit SK)|
|`tanggalBerakhir`|Date|Opsional|Null = masih menjabat;<br>terisi saat ditutup|
|`alasanPerubahan`|Text|Opsional|Contoh: Otomatis<br>demisioner karena SK baru<br>[nomor] telah disetujui;<br>Mutasi ke [jabatan tujuan]|
|`createdAt /`<br>`updatedAt`|DateTime|Otomatis|Jejak waktu|
|`(gabungan)`|Index|UNIQUE (anggotaId,<br>suratKeputusanId,<br>jabatanId)|Cegah duplikasi personalia<br>pada SK yang sama|
|`(gabungan)`|Index|UNIQUE parsial:<br>anggotaId WHERE status<br>= AKTIF|Larangan jabatan ganda<br>(keputusan desain nomor<br>10): satu anggota hanya<br>satu jabatan aktif pada satu<br>waktu, lintas tingkat<br>sekalipun|



###### **Tabel 13. Spesifikasi tabel masterJabatan** 

|**Kolom**|**Tipe**|**Batasan**|**Keterangan**|
|---|---|---|---|
|`id`|Int|PK, autoincrement|Identitas jabatan|
|`namaJabatan`|Varchar(100)|UNIQUE, NOT NULL|Diisi manual; contoh:<br>Ketua Umum, Sekretaris<br>Umum, Bendahara<br>Umum, Ketua Bidang,<br>dst.|
|`isKetuaUmum`|Boolean|NOT NULL, default false|Ditandai manual saat<br>menambah jabatan<br>(keputusan nomor 14);<br>memicu filter etalase<br>publik|
|`urutan`|Int|NOT NULL|Orde tampilan resmi<br>susunan pengurus|
|`dibuatOleh`|Int|FK adminUser.id,<br>opsional|Pencipt jabatan; admin<br>mana pun dapat<br>menambah (tercatat audit)|



16 

Dokumen Desain Teknis SIM KIPAN - v1.1 

|**Kolom**|**Tipe**|**Batasan**|**Keterangan**|
|---|---|---|---|
|`isActive`|Boolean|NOT NULL, default true|Nonaktif = tak lagi<br>muncul di dropdown<br>personalia; riwayat lama<br>tetap utuh|



#### **4.2.5 Domain Akses dan Audit** 

**Tabel 14. Spesifikasi tabel adminUser** 

|**Kolom**|**Tipe**|**Batasan**|**Keterangan**|
|---|---|---|---|
|`id`|Int|PK, autoincrement|Identitas akun admin|
|`email`|Varchar(150)|UNIQUE, NOT NULL|Kredensial utama admin|
|`passwordHash`|Varchar(100)|NOT NULL|bcrypt cost 12; tidak<br>pernah menyimpan kata<br>sandi polos|
|`namaLengkap`|Varchar(150)|NOT NULL|Tampil di audit log dan<br>antarmuka|
|`peran`|Enum|NOT NULL|SUPER_ADMIN /<br>ADMIN_NASIONAL /<br>ADMIN_PROVINSI /<br>ADMIN_KABUPATEN|
|`provinsiId`|Int|FK, opsional|Wajib utk<br>ADMIN_PROVINSI; null<br>utk SUPER_ADMIN dan<br>ADMIN_NASIONAL|
|`kabupatenKotaId`|Int|FK, opsional|Wajib utk<br>ADMIN_KABUPATEN;<br>null utk peran lain|
|`statusAktif`|Boolean|NOT NULL, default true|Nonaktif = akun dibekukan<br>tanpa dihapus|
|`terakhirLogin`|DateTime|Opsional|Jejak keamanan|
|`dibuatOleh`|Int|FK adminUser.id, opsional|Super Admin/Admin<br>Nasional pengundang akun|
|`createdAt /`<br>`updatedAt`|DateTime|Otomatis|Jejak waktu|



**Tabel 15. Spesifikasi tabel auditLog** 

|**Kolom**|**Tipe**|**Batasan**|**Keterangan**|
|---|---|---|---|
|`id`|Int|PK, autoincrement|Identitas log|
|`userId`|Int|FK adminUser.id, opsional|Eksekutor; null utk aksi<br>sistem otomatis|
|`userNama /`<br>`userPeran`|Varchar|NOT NULL|Snapshot identitas saat aksi<br>(kebal terhadap perubahan<br>data akun)|
|`aksi`|Varchar(50)|NOT NULL|Daftar aksi pada Tabel 28;<br>dapat diperluas|



17 

Dokumen Desain Teknis SIM KIPAN - v1.1 

|**Kolom**|**Tipe**|**Batasan**|**Keterangan**|
|---|---|---|---|
|`entitas`|Varchar(40)|NOT NULL|pendaftaran / anggota /<br>suratKeputusan /<br>pengurus / adminUser /<br>berkas|
|`entitasId`|Int|NOT NULL|Baris tujuan aksi|
|`detail`|Jsonb|Opsional|Muatan kontekstual (status<br>sebelum-sesudah, alasan,<br>NIA terkait)|
|`createdAt`|DateTime|Otomatis|Waktu aksi; tabel append-<br>only tanpa<br>UPDATE/DELETE dari<br>aplikasi|



### **4.3 Generator Nomor Induk Anggota (NIA)** 

Sesuai keputusan desain nomor satu, NIA disusun dengan formula: **NIA = KIPANIND-[kodeWilayah]-[nomorUrut]** , dengan kodeWilayah adalah empat digit pertama NIK pendaftar (yang identik dengan kode Kemendagri kabupaten/kota asal KTP) dan nomorUrut adalah enam digit berpembatas nol. Sebagai contoh, pendaftar dengan NIK berawalan 3217 yang merupakan kader pertama di wilayah tersebut memperoleh NIA `KIPAN-IND-3217000001` . Komponen tahun tidak dipakai, sehingga nomor bersifat permanen sepanjang masa keanggotaan dan tidak pernah diterbitkan ulang meskipun anggota keluar. 

Algoritma penerbitan dieksekusi di dalam satu transaksi basis data pada saat admin kab/kota menyetujui pendaftaran agar bebas dari kondisi balapan (race condition) antar dua persetujuan bersamaan: 

1. Baca kunci: ambil baris `niaCounter` milik kabupatenKota target dengan kunci baris (SELECT ... FOR UPDATE) sehingga proses paralel lain menunggu. 

2. Naikkan `lastNumber` sebesar satu lalu susun NIA dengan padding nol enam digit. 

3. Salin data pendaftaran menjadi baris `anggota` berstatus AKTIF beserta relasi berkas foto. 

4. Tautkan `pendaftaran.anggotaId` agar alur pendaftaran dan keanggotaan dapat saling dilacak. 

5. Commit transaksi; kegagalan pada langkah mana pun membatalkan seluruh perubahan (atomic). 

Kapasitas penomoran enam digit menyediakan 999.999 nomor per kabupaten/kota - batas kader resmi per wilayah yang disepakati pemilik produk. Sesuai konfirmasi final (keputusan desain nomor 12), nomor urut dihitung per kabupaten/kota: setiap wilayah mulai dari 000001 secara mandiri, sehingga pencapaian nomor tertinggi pada satu kabupaten/kota tidak memengaruhi penomoran wilayah lain. Kasus khusus: pendaftar dengan domisili 

18 

Dokumen Desain Teknis SIM KIPAN - v1.1 

berbeda dari kode NIK tetap memperoleh NIA sesuai kode NIK sesuai keputusan; peristiwa ini ditandai pada laporan anomali (Bab 8.2) untuk ditinjau organisasi. 

## **5. LOGIKA BISNIS INTI** 

Seluruh logika bisnis pada bab ini melayani satu tujuan manajerial: mengelola pendataan kader sejak pendaftaran, verifikasi, dan pengangkatan menjadi pengurus, hingga dinyatakan demisioner. Gambar 3 merangkum siklus ujung-ke-ujung tersebut beserta dua prinsip penyertanya: personalia SK selalu diambil dari data kader resmi, dan pengangkatan pengurus tidak pernah mengubah data anggota - hanya menambahkan label status kepengurusan serta entri riwayat yang memuat periode bakti. 

19 

###### Dokumen Desain Teknis SIM KIPAN - v1.1 



<!-- Start of picture text -->
Siklus Manajerial Kader: dari Pendaftaran hingga Demisioner<br>Fokus SIM KIPAN — pendatak a dern -+ pengangkatantidak berubahpengurus(non-destrukti)~tata kelola demisioner, dengan Data Anggota yang selalu<br>{ tahop 1 —Penparranan an<br>Mengisi formulir 4 bagian, mengunggah berkes, menyetujui paktaintegrtas, alu<br>Status: DRAFT rmengitim,<br>Soins Sistem menerbitkan nomor registrasi REG-YYYYMM-XXXX; pelacokan status<br>Bieima/DAin est mand via NIK + tanggal lable.<br>Status awal setelah submit: DIKIRIM (tampil sebagai “DiterimayDikirim”)<br>{ Tahap 2 — veniekast& KADER nEsét ESAT<br>Hanya admin kabykota wilayah pendaftar yang memverifikas (satu tahap):<br>Status: DISETUJUI —__memeriksa kelengkapan berkas lal menyetujul menjadi KADER RESMI<br>stem menerbitkan LIND urut per kabykota) dan @-KTA<br>/ DITOLAK sist itkan NIA KIPAN-ND-3217-000003 (urut<br>wip OTOLAX permanen. 89 yge-QR (PNG+ POF, tanda tangan statis Ketua Umum).<br>Data kader masuk ke Data Anggota dengan status AKTIF<br>\<br>(_tehap 3 — PENGANGKATAN PENGURUS (SK) [ADMIN WILAYAH + RANTA PERSETUJUAN<br>Wilayah yang ingin membentuk kepengurusan menyusun SK: personalia dlambll<br>SK DPC/DPD/ DPN dariVinsData YabarKader:18 kabDPC +dari9 kota),kaderDPNkabykota bebassetempat. wilayah DPD dari seluruh kab/kota<br>personae si darcate | RANtal perSetujuan: DPC direview provins! lau disahkan nasional; DPD disahkan<br>Tader eso nasional; DPN langsung efekti<br>Volidas sistem: kader yang sudah menjabat aktif ditolak — tidak ada jabatan<br>ganda.<br>{ tahap 4— menasar (encunus axriy TERT<br>Setelah SK disahkan, personalia tampil i Data Pengurus sesuai wilayah masing<br>‘masing (kabrkota, provinsi, atau nasional)<br>eRe Data Anggota tidak berubah — hanya bertambah label "Sedang menjabat:<br>mee sog_ Pegurus[tingkat!” dan enrriwayat bers jabatan,wilayah, serta periode bakti<br>Selama menjabat terseaia mekanisme PAW (pemberhentian antar waktu) dan<br>‘mutasl(perpindahan jabatan/wilayah; jabatan lama otomatis demisionen,<br>( Tahap 5 — DEMISIONER SISTEM (OTOMATIS) + KADER TETAP ANGGOTA<br>status: Masa bakti SK berakhir (deteksi Kedaluwarsa dinamis + cron harian), SK baru<br>Kolom RIWAYAT menampikan periode teakhir, ontoh: “Demisioner Ketua Umum<br>DEMISIONER dlisahkan, PAW, atau mutasi — jabatan ditutup otomatis sebagai Demisioner.<br>lodertetaptecauracie | Jawa Barat (Perlode 2020-2023)" — bukti pemah menjadi bagian kepengurusan<br>sebagalanggota pada periode tersebut.<br>Kader dapat diajukan kembal pada SK berikutnya kapan pun.<br> Siklus berulang: kader demisioner dapat diajukan Kembali menjadi pengurus pada SK perode berikutnya ~ rwayat jabatan<br>'Sebelumnyatetaptersimpan dan tidak pemah dinapus.<br>Prinsip non-destruktt: pengangkatan maupun pemberhentian pengurus tidak pemah mengubah bars Data Anggota: yong<br>berubah hanyalah record kepengurusan beserta label dan rayat yang dihitung dari elasi sotasl wilayah: setiop admin hanya<br>mengelola wiayan yursclisinya ~ metinat wilayah lain dperbolehkan dalam mode bace-saa<br><!-- End of picture text -->

###### _Gambar 3. Siklus manajerial kader: pendaftaran, verifikasi, kepengurusan, dan demisioner_ 

20 

Dokumen Desain Teknis SIM KIPAN - v1.1 

### **5.1 Mesin Status Pendaftaran dan Verifikasi** 

Pendaftaran kader dikelola sebagai mesin status terpusat dengan lima keadaan: DRAFT, DIKIRIM, PERBAIKAN, DISETUJUI, dan DITOLAK. Sesuai keputusan desain nomor delapan, verifikasi hanya berlangsung **satu tahap** : seluruh putusan berada di tangan Admin Kabupaten/Kota wilayah pendaftar, tanpa keterlibatan admin provinsi. Hanya transisi yang tercantum pada tabel transisi yang diizinkan; seluruh percobaan transisi lain ditolak oleh lapisan layanan dengan galat validasi. Gambar 4 menggambarkan alur lengkapnya dari sudut pandang tiga aktor: calon kader, Admin Kabupaten/Kota, dan sistem otomatis. 



<!-- Start of picture text -->
Alur Pendaftaran Kader & Verifikasi Satu Tahap oleh Admin<br>Wilayah<br>Ungpopesfoto+ feta eT. bare<br>REC rim clue empl seboye-Diarinurm” Sun<br>bertasdemalian, anys beras bermaslah yang dunggah ang tanga meng Ung data<br>6 bl persyaratan englap: menyetulul pendaftar menjadl KADER RESMI “iE — tnpo pers<br>2 MenghasitonNomor ind AggataKIPANAIND-8 digit awa NIK/-Lurt 6 digit per abot]<br><!-- End of picture text -->

_Gambar 4. Alur pendaftaran kader dan verifikasi satu tahap oleh admin wilayah_ 

21 

Dokumen Desain Teknis SIM KIPAN - v1.1 

**Tabel 16. Tabel transisi status pendaftaran** 

|**Status Awal**|**Aksi (Aktor)**|**Status Akhir**|**Efek Samping**|
|---|---|---|---|
|DRAFT|Submit formulir lengkap +<br>pakta integritas (calon<br>kader)|DIKIRIM|Terbit nomor REG-<br>YYYYMM-XXXX; tampil<br>ke pendaftar sebagai<br>Diterima/Dikirim|
|DIKIRIM|Temukan berkas belum<br>lengkap (admin kab/kota<br>wilayahnya)|PERBAIKAN|catatanRevisi terisi; hanya<br>berkas bermasalah yang<br>diunggah ulang|
|PERBAIKAN|Unggah ulang berkas lalu<br>ajukan kembali (calon<br>kader)|DIKIRIM|versi berkas bertambah;<br>biodata tidak diulang|
|DIKIRIM|Setujui karena persyaratan<br>lengkap (admin kab/kota<br>wilayahnya)|DISETUJUI|Kader resmi; pipeline NIA<br>+ e-KTA dijalankan<br>(Subbab 5.2)|
|DIKIRIM|Tolak permanen (admin<br>kab/kota wilayahnya)|DITOLAK|alasanPenolakan wajib<br>terisi; data disimpan<br>permanen, tidak dihapus|
|Semua status|Bypass pemulihan (Super<br>Admin/Admin Nasional)|DISETUJUI|Wajib mencatat alasan<br>bypass pada audit log|



Validasi pra-submit dijalankan pada dua lapis: klien (Zod, pengalaman pengguna) dan server (Zod, keamanan). Usia dihitung otomatis dari tanggal lahir pada saat submit dan harus berada pada rentang 16 hingga 30 tahun. Duplikasi NIK dicegah melalui indeks unik parsial pada kolom nikHash (hanya status non-DITOLAK) sehingga pendaftaran kedua dengan NIK yang sama ditolak basis data selama masih ada pendaftaran berjalan, terlepas dari kondisi aplikasi. Pelacakan status sepenuhnya mandiri melalui menu publik dengan kombinasi NIK + tanggal lahir, karena notifikasi WhatsApp ditunda hingga gateway resmi ditetapkan (keputusan desain nomor 15). Pendaftar yang berstatus DITOLAK tetap dapat mengajukan pendaftaran baru kapan pun; arsip penolakan lama terpelihara sebagai histori tanpa memblokir upaya berikutnya. 

### **5.2 Pipeline Penerbitan NIA dan e-KTA** 

Persetujuan oleh Admin Kabupaten/Kota wilayah pendaftar (atau bypass pemulihan Super Admin/Admin Nasional) memicu pipeline penerbitan yang berjalan sebagai satu kesatuan layanan: 

1. Eksekusi transaksi generator NIA sebagaimana dirinci pada Subbab 4.3. 

2. Konversi data menjadi baris anggota berstatus AKTIF dan penautan dua arah dengan pendaftaran. 

3. Perenderan e-KTA sisi depan: logo resmi KIPAN dan Kemenpora RI, foto resmi kader, nama lengkap (tebal), NIA, NIK tersensor enam digit tengah (pola 5-6-5, contoh 32101*****1234 diganti tanda bintang), wilayah domisili, serta tanggal terbit. 

22 

Dokumen Desain Teknis SIM KIPAN - v1.1 

4. Perenderan sisi belakang: pakta integritas anti narkoba, citra tanda tangan statis Ketua Umum DPN (aset resmi yang telah tersedia; keputusan desain nomor 17), dan QR code verifikasi. 

5. Pembuatan dua berkas keluaran dari satu templat: PNG (siap disimpan di ponsel) dan PDF (satu halaman, siap cetak) dengan dimensi standar kartu 85,6 x 53,98 mm. 

6. QR code dialamatkan ke halaman validasi publik /validasi-kta/[nia] yang menampilkan status keabsahan kader secara waktu nyata. 

7. Pencatatan audit TERBIT_NIA beserta NIA dan nomor registrasi asal, lalu pengiriman notifikasi persetujuan ke pendaftar. 

Anggota dapat mengunduh ulang e-KTA kapan pun dari portal anggota dalam kedua format; perenderan dilakukan on-demand dari data terkini sehingga kartu selalu memuat status terbaru. Halaman validasi publik sengaja tidak menampilkan NIK maupun kontak pribadi, hanya nama, foto, NIA, wilayah, status keanggotaan, dan riwayat jabatan ringkas. 

### **5.3 Siklus Hidup SK dan Single Active SK Rule** 

Rantai persetujuan SK berbeda untuk setiap tingkat kepengurusan: SK tingkat Kabupaten/Kota (DPC) diajukan oleh admin kab/kota lalu harus **direview Admin Provinsi terlebih dahulu dan kemudian disahkan Admin Nasional** ; SK tingkat Provinsi (DPD) cukup disahkan Admin Nasional; sedangkan SK tingkat Nasional (DPN) langsung efektif setelah diinput Admin Nasional tanpa persetujuan pihak lain. Personalia selalu diambil dari data kader resmi sesuai cakupan wilayah - SK DPC hanya boleh memuat kader resmi kabupaten/kota tersebut, SK DPD boleh memuat kader dari seluruh kabupaten/kota pada provinsi tersebut (misalnya Jawa Barat: 18 kabupaten dan 9 kota), dan SK DPN bebas memuat kader dari wilayah mana pun. Pada saat sebuah SK mencapai status DISETUJUI, sistem menjalankan **Single Active SK Rule** : SK lama pada wilayah yang sama dinonaktifkan (statusAktif = false) dan seluruh personalianya otomatis beralih menjadi Demisioner dengan alasan terformat, semuanya di dalam satu transaksi basis data. Gambar 5 merangkum siklus tersebut. 

**Tabel 17. Rantai persetujuan SK per tingkat kepengurusan** 

|**Tingkat SK**<br>Kabupaten/Kota|**Penyusun dan**<br>**Pengaju**<br>Admin Kab/Kota|**Sumber Personalia**<br>Kader resmi|**Rantai Persetujuan**<br>Review Admin|**Efek DISETUJUI**<br>Personalia resmi|
|---|---|---|---|---|
|(DPC)|wilayah|kabupaten/kota|Provinsi, lalu|menjabat; tampil di|
|||tersebut|pengesahan Admin<br>Nasional|Data Pengurus DPC|



23 

Dokumen Desain Teknis SIM KIPAN - v1.1 

|**Tingkat SK**|**Penyusun dan**<br>**Pengaju**|**Sumber Personalia**|**Rantai Persetujuan**|**Efek DISETUJUI**|
|---|---|---|---|---|
|Provinsi (DPD)|Admin Provinsi|Kader resmi seluruh<br>kab/kota pada<br>provinsi (Jabar: 18<br>kab + 9 kota)|Pengesahan Admin<br>Nasional saja|Personalia resmi<br>menjabat; tampil di<br>Data Pengurus DPD|
|Nasional (DPN)|Admin Nasional|Kader resmi wilayah<br>mana pun|Tanpa persetujuan -<br>langsung efektif|Personalia resmi<br>menjabat; tampil di<br>Data Pengurus DPN|



24 

Dokumen Desain Teknis SIM KIPAN - v1.1 



<!-- Start of picture text -->
Siklus Hidup SK: Rantai Persetujuan per Tingkat & Otomatisasi<br>Demisioner<br>Stotus persetuuan: DRAFT -+ MENUNGGU_PROVINSI / MENUNGGU_NASIONAL + DISETUJU/DITOLAK — varan rani<br>{ tehap 1 — Penyusunan brat ‘ADM WILAYAH (DPC /DPD/ DPM)<br># Membuatkepengurusan,usuian danSK unggahbaru: nomor lampiranSK dist berkasmanual SK (wajib)sesuai kebjakan| oRArT sekretait, judul, periode<br>2) Menambahtanvilayah sesuaitingkatpersonalSK (DPC:melaluikab/katasetempat;modal pncarian kaderDPD: seluruh kab/kotabeNIA — hanya dikaderresmiprovinsi; DPN:padabebas) dancakupan<br>Yong tidak sedang menjebat akti aranganjabaten ganda; jabatandiplin dar master jobaten yeng<br>Gikelola manval<br>{ tehap 2— Rantai Persetujuan (Berbeda per Tingkat) —__Aovin pnovins!/ ADMIN WASIONAL<br>3) SK Kabupaten/Kote (DPC): dijukan admin kabiota~ direview & disetujul Admin Provins!<br>44 SK Provinsi (DPD): dajukan admin provinsi ~cukup disahkan Admin Nasional<br>5 SKaT Nasional (DPN): input Admin Nasional -slangsung efektif tanpa persetujuan pihak iain<br>6 Penclakan pada tahap mana pun wa disertalcatatan; dat dapat cisalin ung untuk dlperbak<br>{ tehap 3 — Pengesahan & Pengikatan ADMIN WASIONAL + sisTEM<br>@ ain Nasional mengesahkan final; seluruh persoalia tercantum resmi menjabat dan tempi di Data<br>Pengurus sesval wiloyahnya. oseTyUI<br>{Sistem menonaktikan Sk lama pada wilayah yong sama — beraku Single Active SK Re (sat<br>vilayah hanya satu SK kt)<br>3) Sel personalia SK lama otomatis berlin status dengan log aut: “Otomatis demisioner karena SK<br>baru telah disetuj™ oehsowen<br>4 Deteksikedaluwarsa dinamis: saat tanggal har ini melewatitanggalBeraknir Sk, seluruh pengurustampil<br>sebagai Demisioner meski belum ada SK baru oTowaris<br>{PAW (pengaktiran individual sebelum masa belt berakhin: demisioner purna tgas,diberhentikan<br>{B Mutasi att kader pindah jabotanwilayah sat masa Doki beralan— jobatan ama atomatis Demisoner<br>lebin danuu, rvayat baru cibuka pada SKjabatan tvan (tidak pemnah ada jabatan ganda)<br>Konsekuens Single Active SK Rule: pada sat SK baru berstatus DISETU)U, sistem menonaktfan SK ama (sttusA =<br>transatl Bal data Riayat abatn lama etap tersimpan sehngga olom RIWAYAT dapat menampikan “Demisoner<br>Usbatnt wool (erode) yong tera: Larangan jabatan ganda: sat kader hanya depst menoba at jootan aki<br>{Aturantampilan pubis hanya pengurus dengan jabatan Ketua Umum (OPN/OPD/DPC yang tampitan pada halaman<br><!-- End of picture text -->

_Gambar 5. Siklus hidup SK, rantai persetujuan tiga varian, dan otomatisasi demisioner_ 

25 

Dokumen Desain Teknis SIM KIPAN - v1.1 

**Tabel 18. Tabel transisi status SK dan efek sampingnya** 

|**Status Awal**|**Aksi (Aktor)**|**Status Akhir**|**Efek Samping**|
|---|---|---|---|
|DRAFT (KABUPATEN)|Ajukan usulan (admin<br>kab/kota)|MENUNGGU_PROVINSI|Personalia terkunci; hanya<br>catatan penolakan yang<br>dapat mengubah|
|DRAFT (PROVINSI)|Ajukan usulan (admin<br>provinsi)|MENUNGGU_NASIONA<br>L|Sama seperti di atas|
|MENUNGGU_PROVINSI|Setujui review (admin<br>provinsi)|MENUNGGU_NASIONA<br>L|reviewProvinsiOleh +<br>tanggalReviewProvinsi<br>terisi; menunggu<br>pengesahan nasional|
|MENUNGGU_PROVINSI|Tolak (admin provinsi)|DITOLAK|catatanPenolakan wajib<br>terisi|
|MENUNGGU_NASIONA<br>L|Setujui final (admin<br>nasional)|DISETUJUI|Transaksi Single Active<br>SK + demisioner massal;<br>personalia resmi menjabat|
|MENUNGGU_NASIONA<br>L|Tolak (admin nasional)|DITOLAK|catatanPenolakan wajib<br>terisi|
|DITOLAK|Susun ulang (salin menjadi<br>draf baru)|DRAFT|Nomor SK baru; rujukan<br>SK lama disimpan|
|DRAFT (NASIONAL)|Sahkan langsung (admin<br>nasional)|DISETUJUI|Berlaku ketentuan Single<br>Active SK tingkat nasional|



Urutan transaksi saat pengesahan adalah: validasi periode (tanggalTerbit lebih kecil dari tanggalBerakhir, dan tidak bertumpang tindih dengan SK aktif existing pada wilayah yang sama), validasi akhir larangan jabatan ganda (tidak ada personalia yang sedang menjabat aktif pada SK mana pun), nonaktifkan SK lama, tutup seluruh record pengurus SK lama menjadi DEMISIONER dengan alasan `Otomatis demisioner karena SK baru [nomorSK] telah diterbitkan dan disetujui` , set statusAktif SK baru menjadi true, tulis audit SETUJU_SK dan DEMISIONER_OTOMATIS per personalia, lalu commit. Penolakan tidak pernah mengubah SK lama; wilayah tetap dilayani SK existing hingga ada pengesahan baru. Dua pengaman tambahan ditegakkan sejak tahap draf: pertama, modal pencarian personalia hanya menampilkan kader resmi pada cakupan wilayah sesuai tingkat SK; kedua, penambahan personalia yang masih memiliki jabatan aktif ditolak dengan galat 409. Kedua validasi diulang di dalam transaksi pengesahan agar kebal terhadap pengajuan paralel dari wilayah berbeda. 

Setelah SK disahkan, personalia yang tercantum dinyatakan sebagai pengurus resmi dan otomatis tampil pada Data Pengurus sesuai wilayah kabupaten/kota, provinsi, atau nasional masing-masing. Penting dicatat bahwa pengangkatan ini **tidak mengubah satu pun kolom pada data anggota** : tidak ada salinan nama maupun jabatan yang disisipkan ke tabel anggota. Yang berubah hanyalah kehadiran record kepengurusan baru, sehingga label status (misalnya Sedang menjabat: Pengurus Kabupaten/Kota) dan riwayat periode pada tampilan anggota dihitung dari relasi pengurus secara waktu nyata. Pendekatan non- 

26 

Dokumen Desain Teknis SIM KIPAN - v1.1 

destruktif ini menjaga Data Anggota selalu bersih sebagai sumber tunggal identitas kader, dan riwayat lengkap tetap dapat direkonstruksi kapan pun. 

### **5.4 Deteksi Kedaluwarsa Dinamis (Dynamic Demisioner Check)** 

Kedaluwarsa masa bakti ditangani pada dua lapis yang saling melengkapi. Lapis tampilan: setiap kali kolom RIWAYAT atau daftar pengurus dirender, sistem menghitung **status efektif** = record berstatus AKTIF dan SK terkait berstatusAktif = true dan tanggalBerakhir SK belum terlewat. Apabila tanggal hari ini telah melewati tanggalBerakhir, pengurus ditampilkan sebagai Demisioner meskipun belum ada SK pengganti dan belum ada proses manual apa pun. Lapis materialisasi: worker cron harian menutup record yang kedaluwarsa (menetapkan status DEMISIONER, tanggalBerakhir, dan alasan) sehingga laporan dan statistik tidak perlu menghitung ulang. Pendekatan ganda ini menjaga antarmuka tetap responsif terhadap pergantian tanggal sekaligus menjaga konsistensi data jangka panjang. 

### **5.5 Algoritma Tiga Kolom Riwayat** 

Tabel master data anggota wajib menampilkan tiga kolom identitas utama. Kolom **NAMA** menampilkan nama lengkap tebal dengan NIA di bawahnya; kolom **PEKERJAAN** menampilkan profesi saat mendaftar dan bernilai tanda hubung apabila kosong; kolom **RIWAYAT** mengikuti algoritma prioritas berikut: 

```
fungsi hitungRiwayat(anggota):
  riwayat = semua record pengurus milik anggota
  jika riwayat kosong:
      kembalikan "-"                       // kader biasa
  aktif = riwayat dengan status AKTIF
          DAN SK.statusAktif = true
          DAN SK.tanggalBerakhir >= hari ini
  jika aktif tidak kosong:
      r = anggota aktif terbaru (urut tanggalMulai)
      kembalikan "Pengurus Aktif {r.jabatan} {r.wilayah} (Periode
{r.periodeBakti})"
  r = riwayat terakhir berdasarkan tanggalMulai
  kembalikan "Demisioner {r.jabatan} {r.wilayah} (Periode {r.periodeBakti})"
```

Aturan kuncinya sederhana namun ketat: jabatan aktif selalu menang atas histori berapa pun panjangnya, dan bila seluruh riwayat telah berakhir, yang ditampilkan adalah **riwayat terakhir** , bukan yang terlama. Komponen periodeBakti dihitung dari SK terkait sebagai rentang tahun tanggalTerbit hingga tanggalBerakhir (contoh: 2023-2026) sehingga setiap baris riwayat selalu tercantum periodenya. Tiga kasus uji mengikuti skenario pada URD dan wajib ada dalam rencana pengujian: 

27 

Dokumen Desain Teknis SIM KIPAN - v1.1 

**Tabel 19. Kasus uji algoritma riwayat (skenario kader bernama Mirwan)** 

|**Kasus**|**Data Riwayat**|**Tampilan Kolom RIWAYAT**|
|---|---|---|
|1. Sedang menjabat di provinsi|Ketua Umum Kab. Bandung Barat<br>(Periode 2017-2020, Demisioner);<br>Ketua Umum Prov. Jawa Barat<br>(Periode 2023-2026, Aktif)|Pengurus Aktif Ketua Umum Jawa<br>Barat (Periode 2023-2026)|
|2. Purna tugas di dua tingkat|Ketua Umum Kab. Bandung Barat<br>(Periode 2017-2020); lalu Ketua<br>Umum Jawa Barat (Periode 2020-<br>2023); keduanya berakhir|Demisioner Ketua Umum Jawa Barat<br>(Periode 2020-2023)|
|3. Kader biasa|Belum pernah tercantum dalam SK<br>pengurus mana pun|- (tanda hubung tegas)|



**Label dan riwayat yang non-destruktif:** ketika seorang kader diangkat menjadi pengurus, baris data anggotanya tidak berubah - tampilan hanya menambahkan label kepengurusan aktif (tingkat kabupaten/kota, provinsi, atau nasional) beserta riwayat yang memuat jabatan, wilayah, dan periode bakti. Setelah dinyatakan demisioner, riwayat itulah yang menunjukkan bahwa kader bersangkutan pernah menjadi bagian dari kepengurusan pada periode tertentu, misalnya "Ketua Umum Jawa Barat (Periode 20202023)". 

### **5.6 Pergantian Antar Waktu (PAW) dan Mutasi Jabatan Aktif** 

Sebelum masa bakti berakhir, admin yang berwenang dapat mengubah status pengurus perorangan melalui empat aksi pengakhiran PAW: demisioner purna tugas lebih awal, diberhentikan (sanksi pelanggaran AD/ART), mengundurkan diri (berdasarkan surat resmi), dan meninggal dunia (purna tugas permanen). Selain keempat aksi tersebut, sesuai keputusan desain nomor tiga tersedia pula **mutasi aktif** : perpindahan jabatan atau wilayah selama masa bakti masih berjalan, misalnya kader dipromosikan dari pengurus cabang ke pengurus daerah. 

**Tabel 20. Aksi PAW dan mutasi beserta efeknya pada record pengurus** 

|**Aksi**|**Efek pada Record Lama**|**Efek Lain**|**Wewenang**|
|---|---|---|---|
|Demisioner lebih awal|Ditutup: status<br>DEMISIONER +<br>tanggalBerakhir hari ini|Alasan purna tugas tercatat|Admin wilayah setingkat<br>atau di atasnya|
|Diberhentikan|Ditutup: status<br>DIBERHENTIKAN|Menyalakan penanda<br>sanksi pada profil anggota<br>utk peninjauan organisasi|Admin provinsi/nasional|
|Mengundurkan diri|Ditutup: status<br>MENGUNDURKAN_DIR<br>I|Lampiran surat<br>pengunduran diri<br>diarsipkan|Admin wilayah setingkat<br>atau di atasnya|
|Meninggal dunia|Ditutup: status<br>MENINGGAL_DUNIA|statusAnggota anggota<br>juga menjadi<br>MENINGGAL; e-KTA<br>ditandai tidak berlaku|Admin setingkat apa pun +<br>verifikasi nasional|



28 

Dokumen Desain Teknis SIM KIPAN - v1.1 

|**Aksi**|**Efek pada Record Lama**|**Efek Lain**|**Wewenang**|
|---|---|---|---|
|Mutasi aktif|Ditutup: status|Record pengurus baru|Admin wilayah tujuan +|
||DEMISIONER, alasan|dibuka pada SK/jabatan|persetujuan berjenjang bila|
||Mutasi ke [jabatan]|tujuan dengan|lintas tingkat|
||[wilayah]|tanggalMulai hari ini||



Pada seluruh aksi, record lama tidak pernah dihapus sehingga histori jabatan tetap utuh dan algoritma tiga kolom tetap bekerja dengan benar; mutasi lintas SK yang belum disahkan tidak diperkenankan dan harus melalui pengesahan SK PAW terlebih dahulu. Karena larangan jabatan ganda (keputusan desain nomor 10), mutasi selalu menutup record lama lebih dahulu sebelum membuka record baru dalam satu transaksi - tidak ada titik waktu ketika seorang anggota memiliki dua jabatan aktif sekaligus. Setiap aksi PAW dan mutasi wajib menghasilkan entri audit dengan pelaku, waktu, alasan, dan nomor SK terkait. 

## **6. SPESIFIKASI API** 

### **6.1 Konvensi Umum** 

Seluruh endpoint menggunakan gaya REST di bawah awalan `/api` dan mengembalikan JSON. Respons sukses mengikuti pola `{ success: true, data: ..., message: "..." }` , sedangkan galat mengikuti `{ success: false, error: { code, message } }` dengan kode HTTP yang semantik (400 validasi, 401 tanpa/invalid token, 403 melanggar scoping wilayah, 404 tidak ditemukan, 409 konflik status, 422 transisi ilegal, 500 galat server). Autentikasi memakai header `Authorization: Bearer <JWT>` dengan masa berlaku token 8 jam untuk admin dan 12 jam untuk anggota; token memuat klaim peran, provinsiId, kabupatenKotaId, dan anggotaId bila relevan. 

Middleware scoping berjalan sebelum handler: klaim wilayah disuntikkan ke filter kueri sehingga, sebagai contoh, Admin Kabupaten Bandung secara teknis tidak dapat membaca antrean pendaftar Kabupaten Bandung Barat walau menyuntikkan parameter secara manual. Daftar bergaya tabel mendukung paginasi standar `?page=1&limit=20` dengan batas atas 100 baris per halaman dan parameter pencarian `?q=` yang dicocokkan pada nama, NIA, atau nomor registrasi. Seluruh parameter unggahan divalidasi ulang di server sebelum diproses. 

### **6.2 API Autentikasi** 

**Tabel 21. Endpoint autentikasi dua jalur** 

|**Metode**|**Endpoint**|**Peran**|**Deskripsi**|
|---|---|---|---|
|POST|`/api/auth/admin/`<br>`login`|Publik|Login admin dengan email<br>+ kata sandi;|
||||mengembalikan JWT<br>klaim admin|



29 

Dokumen Desain Teknis SIM KIPAN - v1.1 

|**Metode**|**Endpoint**|**Peran**|**Deskripsi**|
|---|---|---|---|
|POST|`/api/auth/`<br>`anggota/login`|Publik|Login anggota dengan NIK<br>+ tanggal lahir;<br>mengembalikan JWT<br>klaim anggota|
|GET|`/api/auth/me`|Semua terautentikasi|Memuat identitas sesi<br>token aktif untuk hidrasi<br>UI|



### **6.3 API Publik: Pendaftaran dan Pelacakan Status** 

**Tabel 22. Endpoint publik pendaftaran kader** 

|**Metode**|**Endpoint**|**Peran**|**Deskripsi**|
|---|---|---|---|
|GET|`/api/publik/`<br>`wilayah/provinsi`|Publik|Daftar provinsi untuk<br>dropdown formulir|
|GET|`/api/publik/`<br>`wilayah/kabupaten?`<br>`provinsiId=`|Publik|Daftar kab/kota pada<br>provinsi terpilih|
|POST|`/api/publik/`<br>`pendaftaran`|Publik|Mengirim pendaftaran baru<br>(multipart: biodata +<br>berkas); memvalidasi usia,<br>NIK unik, pakta integritas,<br>dan berkas wajib|
|GET|`/api/publik/cek-`<br>`status?`<br>`nik=&tanggalLahir=`|Publik|Pelacakan status<br>pendaftaran beserta catatan<br>revisi/alasan penolakan|
|POST|`/api/publik/`<br>`pendaftaran/`<br>`[nomorRegistrasi]/`<br>`berkas`|Publik + bukti kepemilikan|Unggah ulang berkas saat<br>PERBAIKAN tanpa<br>mengulang biodata (versi<br>bertambah)|
|GET|`/api/publik/`<br>`validasi-kta/[nia]`|Publik|Validasi keaslian e-KTA<br>waktu nyata untuk QR<br>code|



### **6.4 API Verifikasi dan Putusan Kader** 

**Tabel 23. Endpoint verifikasi satu tahap oleh admin wilayah** 

|**Metode**|**Endpoint**|**Peran**|**Deskripsi**|
|---|---|---|---|
|GET|`/api/verifikasi/`<br>`antrian?page=&q=`|Admin Kab/Kota|Antrean pendaftar berstatus<br>DIKIRIM/PERBAIKAN<br>pada kab/kota milik sesi|
|GET|`/api/verifikasi/`<br>`[id]`|Admin Kab/Kota; Provinsi<br>(baca); Nasional (baca)|Rincian pendaftaran +<br>berkas untuk peninjauan|
|PUT|`/api/verifikasi/`<br>`[id]/putusan`|Admin Kab/Kota|Putusan satu tahap: setujui<br>(memicu pipeline NIA + e-<br>KTA), minta perbaikan<br>(PERBAIKAN + catatan),<br>atau tolak permanen<br>(alasan wajib)|



30 

Dokumen Desain Teknis SIM KIPAN - v1.1 

|**Metode**|**Endpoint**|**Peran**|**Deskripsi**|
|---|---|---|---|
|PUT|`/api/verifikasi/`|Super Admin, Admin|Persetujuan pemulihan|
||`[id]/bypass`|Nasional|lintas tahap; alasan wajib<br>dan teraudit khusus|



### **6.5 API Anggota dan e-KTA** 

**Tabel 24. Endpoint master anggota dan kartu anggota** 

|**Metode**|**Endpoint**|**Peran**|**Deskripsi**|
|---|---|---|---|
|GET|`/api/anggota?`<br>`page=&q=&wilayah=`|Admin (scoped), Nasional|Daftar anggota dengan tiga<br>kolom wajib:<br>NAMA+NIA,<br>PEKERJAAN, RIWAYAT<br>terakhir|
|GET|`/api/anggota/[id]`|Admin (scoped), pemilik<br>akun|Rincian profil anggota|
|PATCH|`/api/anggota/`<br>`[id]/status`|Admin Nasional|NONAKTIF /<br>DIBERHENTIKAN /<br>MENINGGAL; alasan<br>wajib teraudit|
|GET|`/api/anggota/`<br>`[id]/ekta?`<br>`format=png|pdf`|Pemilik akun (anggota),<br>Admin|Unduhan e-KTA; dirender<br>on-demand dari data terkini|
|GET|`/api/anggota/`<br>`search?q=`|Admin (scoped)|Pencarian cepat kader ber-<br>NIA untuk modal<br>personalia SK (nama+NIA,<br>wilayah, riwayat terakhir)|



### **6.6 API Surat Keputusan dan Personalia** 

**Tabel 25. Endpoint pengelolaan SK** 

|**Metode**|**Endpoint**|**Peran**|**Deskripsi**|
|---|---|---|---|
|GET|`/api/sk?`<br>`level=&status=&wil`<br>`ayah=`|Admin (scoped)|Daftar SK sesuai<br>tingkat/wilayah sesi;<br>default view per peran|
|POST|`/api/sk`|Admin Kab/Kota (DPC),<br>Provinsi (DPD), Nasional<br>(DPN)|Membuat draf SK: nomor<br>diisi manual, judul,<br>periode, unggah lampiran<br>(wajib)|
|GET|`/api/sk/[id]`|Admin (scoped)|Rincian SK beserta<br>personalia|
|PUT|`/api/sk/[id]`|Admin penyusun (DRAFT)|Menyunting draf sebelum<br>diajukan|
|POST|`/api/sk/[id]/`<br>`personalia`|Admin penyusun (DRAFT)|Menambah pengurus dari<br>pencarian kader ber-NIA +<br>jabatan; menolak kader di<br>luar cakupan wilayah<br>tingkat SK atau yang<br>sedang menjabat aktif (anti<br>double jabatan)|



31 

Dokumen Desain Teknis SIM KIPAN - v1.1 

|**Metode**|**Endpoint**|**Peran**|**Deskripsi**|
|---|---|---|---|
|DELETE|`/api/sk/[id]/`<br>`personalia/`<br>`[pengurusId]`|Admin penyusun (DRAFT)|Menghapus personalia dari<br>draf|
|PUT|`/api/sk/[id]/`<br>`ajukan`|Admin penyusun|Mengirim usulan: SK<br>KABUPATEN menuju<br>MENUNGGU_PROVINSI<br>; SK PROVINSI menuju<br>MENUNGGU_NASIONA<br>L; SK NASIONAL<br>langsung DISETUJUI|
|PUT|`/api/sk/[id]/`<br>`review`|Admin Provinsi (usulan<br>kab/kota)|Review tahap 1: setujui<br>(menuju<br>MENUNGGU_NASIONA<br>L) atau tolak (catatan<br>wajib)|
|PUT|`/api/sk/[id]/`<br>`persetujuan`|Admin Nasional (usulan<br>kab/kota dan provinsi)|Pengesahan final (memicu<br>Single Active SK +<br>demisioner massal) atau<br>penolakan|



### **6.7 API Pengurus, PAW, dan Mutasi** 

**Tabel 26. Endpoint pengelolaan pengurus dan status** 

|**Metode**|**Endpoint**|**Peran**|**Deskripsi**|
|---|---|---|---|
|GET|`/api/pengurus?`<br>`wilayah=&level=&st`<br>`atus=`|Admin (scoped)|Daftar pengurus dengan<br>default view sesuai peran +<br>filter wilayah untuk<br>provinsi/nasional|
|GET|`/api/pengurus/`<br>`ketua-umum`|Publik|Pimpinan etalase publik:<br>hanya jabatan Ketua<br>Umum DPN/DPD/DPC<br>yang aktif|
|PUT|`/api/pengurus/`<br>`[id]/paw`|Admin berwenang|Pengakhiran individual:<br>demisioner awal /<br>diberhentikan /<br>mengundurkan diri /<br>meninggal dunia|
|PUT|`/api/pengurus/`<br>`[id]/mutasi`|Admin berwenang|Pindah jabatan/wilayah<br>saat aktif; record lama<br>ditutup otomatis sebagai<br>Demisioner|



### **6.8 API Pendukung** 

**Tabel 27. Endpoint statistik, audit, dan administrasi akun** 

|**Metode**|**Endpoint**|**Peran**|**Deskripsi**|
|---|---|---|---|
|GET|`/api/statistik/`<br>`dashboard`|Admin (scoped)|Ringkasan: pendaftar<br>menunggu, anggota aktif,|
||||pengurus aktif, SK berjalan|



32 

Dokumen Desain Teknis SIM KIPAN - v1.1 

|**Metode**|**Endpoint**|**Peran**|**Deskripsi**|
|---|---|---|---|
|GET|`/api/statistik/`<br>`demografi`|Admin Nasional|Demografi anggota<br>nasional (usia, pendidikan,<br>pekerjaan, wilayah) +<br>laporan anomali NIA|
|GET|`/api/audit?`<br>`aksi=&entitas=&dar`<br>`i=&ke=`|Admin Nasional|Jejak audit terfilter; ekspor<br>CSV|
|GET/POST/PATCH|`/api/admin/users`|Super Admin (semua<br>akun); Admin Nasional<br>(admin provinsi/kab)|Kelola akun admin:<br>undang, nonaktifkan, reset<br>kata sandi|
|GET|`/api/master/`<br>`jabatan`|Semua admin|Daftar masterJabatan untuk<br>dropdown personalia SK|
|POST|`/api/master/`<br>`jabatan`|Semua admin|Menambah jabatan baru<br>secara manual (keputusan<br>nomor 14); tercatat audit<br>TAMBAH_JABATAN|
|PATCH|`/api/master/`<br>`jabatan/[id]`|Super Admin, Admin<br>Nasional|Mengubah urutan tampilan,<br>penandaan isKetuaUmum,<br>dan penonaktifan jabatan|



### **6.9 Contoh Muatan Dua Endpoint Kunci** 

Dua contoh berikut memperlihatkan bentuk konkret permintaan dan respons sebagai acuan implementasi klien dan pengujian kontrak API. Contoh pertama adalah pengiriman pendaftaran, contoh kedua adalah daftar anggota yang menegaskan bentuk tiga kolom wajib. 

##### **Contoh 1 - POST /api/publik/pendaftaran** (ringkas): 

```
POST /api/publik/pendaftaran  (Content-Type: multipart/form-data)
namaLengkap=Mirwan&nik=3217xxxxxxxxxxxx&tanggalLahir=2001-05-17
kabupatenKotaId=3217&pekerjaan=Mahasiswa&paktaIntegritas=true
berkas: pas_foto.jpg (image/jpeg, 480 KB), foto_ktp.jpg (image/jpeg, 612 KB)
```

```
200 OK
{ "success": true,
  "data": { "nomorRegistrasi": "REG-202610-0042",
            "status": "DIKIRIM",
            "tanggalDiajukan": "2026-10-04T09:12:33+07:00" },
  "message": "Pendaftaran berhasil dikirim dan menunggu verifikasi admin
wilayah Anda" }
```

**Contoh 2 - GET /api/anggota?page=1&limit=20** (sesi Admin Provinsi Jawa Barat): 

```
200 OK
{ "success": true,
  "data": {
    "items": [
      { "id": 88, "nama": "Mirwan", "nia": "KIPAN-IND-3217-000001",
        "pekerjaan": "Mahasiswa",
        "riwayat": "Pengurus Aktif Ketua Umum Jawa Barat (Periode 2023-
2026)" },
      { "id": 91, "nama": "Siti Rahma", "nia": "KIPAN-IND-3217-000002",
        "pekerjaan": "Wiraswasta", "riwayat": "Demisioner Sekretaris Umum
Kab. Bandung Barat (Periode 2020-2023)" },
```

33 

Dokumen Desain Teknis SIM KIPAN - v1.1 

```
      { "id": 97, "nama": "Adi Nugraha", "nia": "KIPAN-IND-3273-000001",
        "pekerjaan": "PNS", "riwayat": "-" }
    ],
```

```
    "page": 1, "limit": 20, "total": 3 } }
```

Perhatikan bahwa respons sengaja ringkas: NIK tidak pernah ikut daftar, dan kolom riwayat sudah dihitung server dengan algoritma Subbab 5.5 sehingga klien tidak melakukan logika bisnis apa pun. Bentuk ini juga menjadi kontrak pengujian regresi ketika aturan riwayat mengalami perubahan di masa depan. 

## **7. KEAMANAN DATA DAN KEPATUHAN UU PDP** 

### **7.1 Perlindungan NIK: Enkripsi Dua Arah AES-256** 

NIK adalah data pribadi spesifik (specific personal data) dalam pengertian UU Perlindungan Data Pribadi sehingga wajib diamankan pada lapisan penyimpanan. SIM KIPAN menerapkan enkripsi dua arah **AES-256-GCM** untuk kolom nikTerenkripsi pada tabel pendaftaran dan anggota: setiap nilai dienkripsi dengan vektor inisialisasi (IV) 12 byte yang acak per baris dan tag autentikasi GCM, lalu disimpan sebagai base64. Enkripsi GCM dipilih karena sekaligus menjamin kerahasiaan dan integritas ciphertext (tamper-evident). 

Karena nilai terenkripsi tidak dapat dicari langsung, setiap tabel dilengkapi kolom `nikHash` berisi SHA-256 dari NIK dengan indeks unik. Hash ini menjadi kunci deteksi duplikasi pendaftaran dan pencarian cepat tanpa membuka enkripsi. Kunci enkripsi master (KEK) disimpan sebagai variabel lingkungan terpisah dari basis data, idealnya pada key management service; prosedur rotasi kunci tahunan disiapkan dengan pola enkripsi ulang latar belakang. Dekripsi hanya dilayani bagi Admin Kabupaten/Kota dan Admin Provinsi pada wilayah pendaftar serta Admin Nasional, dan **setiap tindakan dekripsi ditulis ke audit log** sehingga penyalahgunaan dapat ditelusuri. NIK tidak pernah dikirim ke log aplikasi, telemetri, atau respons API daftar; pada e-KTA NIK tampil tersensor enam digit tengah (pola 5-6-5). 

### **7.2 Jejak Audit Komprehensif** 

Tabel auditLog bersifat append-only: aplikasi hanya memiliki hak INSERT, tanpa UPDATE maupun DELETE. Setiap aksi kritis mencatat identitas eksekutor (nama dan peran salinan), jenis aksi, entitas sasaran, detail JSON, dan waktu kejadian. Antarmuka penelusuran audit tersedia bagi Admin Nasional dengan filter aksi, entitas, dan rentang waktu, termasuk ekspor CSV untuk pemeriksaan eksternal. Retensi log minimal lima tahun mengikuti kebijakan organisasi dan menjadi dasar akuntabilitas pengurus. Contoh entri log otomatis saat pengesahan SK: 

> `{ "id": 10421, "userId": 12, "userNama": "Ahmad Fauzi", "userPeran":` 

> `"ADMIN_NASIONAL",` 

34 

Dokumen Desain Teknis SIM KIPAN - v1.1 

```
  "aksi": "SETUJU_SK", "entitas": "suratKeputusan", "entitasId": 33,
  "detail": { "nomorSK": "SK-021/DPC-BBR/X/2026", "level": "KABUPATEN",
              "reviewProvinsiOleh": "Rizki Pratama",
              "skLamaDinonaktifkan": "SK-009/DPC-BBR/I/2023",
              "pengurusDemisionerOtomatis": 12 },
  "createdAt": "2026-10-04T14:02:11+07:00" }
```

**Tabel 28. Daftar aksi teraudit beserta pemicunya** 

|**Aksi**|**Pemicu**|**Data Penting yang Tercatat**|
|---|---|---|
|MINTA_PERBAIKAN /<br>SETUJUI_KADER /<br>TOLAK_KADER|Putusan satu tahap admin kab/kota<br>wilayah pendaftar|Catatan revisi / alasan penolakan,<br>status sebelum-sesudah|
|BYPASS_VERIFIKASI|Super Admin/Admin Nasional<br>menyetujui langsung|Alasan bypass (wajib diisi)|
|TERBIT_NIA|Pipeline persetujuan kader|NIA, nomor registrasi asal|
|AJUKAN_SK / REVIEW_SK /<br>SETUJU_SK / TOLAK_SK|Rantai persetujuan SK (review<br>provinsi, pengesahan nasional)|Nomor SK, catatan penolakan,<br>pengesah|
|DEMISIONER_OTOMATIS|Single Active SK dan cron<br>kedaluwarsa|Nomor SK baru/lama, jumlah<br>personalia|
|PAW / MUTASI|Perubahan status pengurus individual|Jenis aksi, alasan, jabatan tujuan|
|TAMBAH_JABATAN|Penambahan jabatan pada<br>masterJabatan oleh admin|Nama jabatan, penanda<br>isKetuaUmum, pelaku|
|LIHAT_LINTAS_WILAYAH|Admin membuka data wilayah lain<br>dalam mode baca-saja|Wilayah sasaran, waktu akses|
|LOGIN / LOGIN_GAGAL|Autentikasi kedua jalur|Waktu, IP, kredensial (tanpa kata<br>sandi/NIK polos)|
|DEKRIPSI_NIK|Pembukaan NIK oleh admin<br>berwenang|Pelaku, entitas sasaran, alasan akses|



### **7.3 Keamanan Berkas dan Validasi Unggahan** 

Berkas unggahan (pas foto, foto e-KTP, berkas SK) diperlakukan sebagai data pribadi. Validasi berlapis diterapkan pada setiap unggahan: pemeriksaan ekstensi dan MIME type terhadap daftar putih (image/jpeg, image/png, application/pdf), pemeriksaan **magic number** isi berkas agar berkas menyamar ditolak, pembatasan ukuran maksimum 2 MB, dan penggantian nama berkas menjadi UUID acak sebelum disimpan pada penyimpanan privat di luar akar publik. Akses berkas hanya melalui endpoint terautentikasi yang mengalirkan isi berkas per permintaan dengan pemeriksaan scoping, bukan URL langsung; berkas pendaftar hanya dapat dibuka oleh verifikator wilayah yang berhak. Untuk keamanan tambahan, hasil unggahan dapat melewati pemindaian malware opsional pada tingkat infrastruktur. 

**Tabel 29. Pemetaan prinsip UU PDP ke implementasi SIM KIPAN** 

|**Prinsip UU PDP**|**Implementasi**|
|---|---|
|Persetujuan dan tujuan jelas|Pakta integritas dan pernyataan persetujuan pemrosesan<br>data pada formulir; data hanya utk keperluan keanggotaan|



35 

Dokumen Desain Teknis SIM KIPAN - v1.1 

|**Prinsip UU PDP**|**Implementasi**|
|---|---|
|Minimisasi data|Hanya data pada URD yang dikumpulkan; tidak ada<br>permintaan data sensitif lain di luar kebutuhan verifikasi|
|Keamanan pengolahan|Enkripsi NIK, bcrypt kata sandi, scoping wilayah,<br>validasi berkas berlapis (Subbab 7.1 dan 7.3)|
|Akuntabilitas dan keterlacakan|Audit log append-only atas seluruh aksi kritis termasuk<br>dekripsi (Subbab 7.2)|
|Hak subjek data|Portal anggota menampilkan data miliknya; permintaan<br>perbaikan/koreksi melalui kanal organisasi tercatat|
|Masa retensi terkendali|Retensi log minimal 5 tahun; data pendaftar DITOLAK<br>disimpan permanen sesuai keputusan organisasi dengan<br>akses dibatasi dan teraudit (keputusan desain nomor 16)|



### **7.4 Keaslian e-KTA dan Tanda Tangan Statis** 

Sisi belakang e-KTA memuat **citra tanda tangan statis Ketua Umum DPN** yang telah tersedia sebagai aset resmi organisasi (keputusan desain nomor 17), bukan tanda tangan elektronik tersertifikasi sebagaimana diatur peraturan tanda tangan elektronik. Konsekuensinya, keabsahan kartu tidak bergantung pada tanda tangan tersebut, melainkan pada **QR code yang menuju halaman validasi publik** /validasi-kta/[nia] - mekanisme yang justru lebih kuat karena memeriksa status keanggotaan secara waktu nyata langsung dari basis data, termasuk penandaan bila anggota telah diberhentikan atau meninggal dunia. 

Aset tanda tangan diperlakukan sebagai berkas sensitif: disimpan pada penyimpanan privat di luar akar publik, hanya dapat dibaca oleh generator e-KTA, tidak dapat diunduh melalui antarmuka mana pun, dan setiap penggantian versi aset dicatat pada audit log beserta identitas pelakunya. Apabila kelak organisasi memutuskan beralih ke tanda tangan elektronik tersertifikasi, generator e-KTA cukup mengganti komponen aset tanpa mengubah struktur data maupun alur penerbitan NIA, sehingga keputusan hari ini tidak menutup opsi peningkatan di masa depan. 

## **8. REKOMENDASI IMPLEMENTASI DAN LANGKAH LANJUT** 

### **8.1 Urutan Implementasi Bertahap** 

Implementasi disarankan berlangsung dalam empat fase berurutan yang masing-masing menghasilkan keluaran yang dapat didemonstrasikan kepada pemilik produk. Fase disusun berdasarkan ketergantungan teknis: skema dan autentikasi menjadi fondasi; pendaftaranverifikasi menghasilkan data anggota; SK-kepengurusan mengonsumsi anggota; dan kartulaporan melengkapi pengalaman akhir. Durasi bersifat indikatif untuk satu tim tiga orang (dua pengembang, satu penguji). 

36 

Dokumen Desain Teknis SIM KIPAN - v1.1 

**Tabel 30. Rencana empat fase implementasi** 

|**Fase**|**Durasi**|**Cakupan Utama**|**Keluaran yang**<br>**Didemonstrasikan**|
|---|---|---|---|
|1. Fondasi|2 minggu|Skema Prisma 11 tabel +<br>migrasi, seed wilayah<br>nasional (master jabatan<br>diisi manual oleh admin),<br>autentikasi dua jalur +<br>middleware scoping,<br>kerangka audit|Login empat peran admin<br>+ anggota berjalan; scoping<br>teruji dengan uji penetrasi<br>sederhana|
|2. Rekrutmen|3 minggu|Formulir 4 bagian, unggah<br>berkas + validasi, cek<br>status publik, antrean<br>verifikasi satu tahap +<br>putusan<br>setujui/perbaikan/tolak,<br>generator NIA<br>transaksional|Alur lengkap pendaftar<br>fiktif hingga DISETUJUI<br>dengan NIA benar; data<br>DITOLAK tetap tersimpan<br>arsip|
|3. Kepengurusan|3 minggu|CRUD SK + personalia via<br>modal pencarian, rantai<br>persetujuan tiga varian per<br>tingkat, Single Active SK +<br>demisioner otomatis,<br>larangan jabatan ganda,<br>PAW + mutasi, cron<br>kedaluwarsa|Pengesahan SK baru<br>mendemosionerkan SK<br>lama; usulan personalia<br>ganda ditolak; kasus<br>Mirwan 1-3 lolos uji|
|4. Penyempurnaan|2 minggu|e-KTA PNG + PDF ber-<br>QR, halaman validasi<br>publik, statistik + laporan<br>anomali, penelusuran audit<br>UI, penguatan keamanan|Unduh kartu dua format;<br>QR tervalidasi; dashboard<br>statistik provinsi|



### **8.2 Risiko Implementasi dan Mitigasi** 

Risiko-risiko berikut diidentifikasi sejak tahap desain beserta langkah mitigasinya agar tidak menjadi kejutan pada fase pembangunan: 

**Tabel 31. Register risiko implementasi dan mitigasi** 

|**Risiko**|**Dampak**|**Mitigasi**|
|---|---|---|
|Kode NIK pendaftar tidak cocok<br>dengan domisili pilihan (NIK<br>lama/pendatang)|NIA memuat kode wilayah asal KTP,<br>bukan domisili aktif|Sesuai keputusan desain; laporan<br>anomali statistik + catatan verifikator<br>utk peninjauan organisasi|
|Balapan nomor urut NIA saat dua<br>persetujuan bersamaan pada<br>kabupaten/kota yang sama|Nomor ganda / gagal terbit|Kunci baris niaCounter dalam<br>transaksi (SELECT FOR UPDATE)<br>+ uji beban konkurensi|
|Dua SK aktif pada satu wilayah<br>karena galat tengah transaksi|Melanggar Single Active SK Rule|Seluruh efek pengesahan dalam satu<br>transaksi atomik + constraint dan uji<br>transaksi batal|
|Penyalahgunaan bypass Super<br>Admin/Admin Nasional|Kader lolos tanpa verifikasi atau SK<br>melompati rantai|Alasan bypass wajib + audit khusus +<br>rekap bulanan bypass utk<br>pengawasan internal|



37 

Dokumen Desain Teknis SIM KIPAN - v1.1 

|**Risiko**|**Dampak**|**Mitigasi**|
|---|---|---|
|Notifikasi WhatsApp belum aktif<br>(gateway ditunda)|Pendaftar tidak mendapat<br>pemberitahuan otomatis|Pelacakan mandiri via NIK + tanggal<br>lahir; hook notifikasi telah disiapkan<br>agar gateway dapat dipasang<br>kemudian tanpa desain ulang|
|Pengajuan personalia ganda secara<br>paralel lintas SK|Seorang kader tercatat menjabat dua<br>jabatan sekaligus|Validasi saat tambah personalia +<br>partial unique index (anggotaId<br>WHERE AKTIF) + pengulangan<br>validasi dalam transaksi pengesahan|
|Retensi permanen data DITOLAK<br>menambah beban penyimpanan|Biaya bertambah; arsip data pribadi<br>menumpuk|NIK tetap terenkripsi + hash, akses<br>arsip dibatasi (verifikator wilayah,<br>Super Admin, Admin Nasional) dan<br>teraudit; peninjauan kebijakan<br>berkala bersama organisasi|
|Kebocoran kunci enkripsi NIK|Data pribadi terpapar|KEK terpisah dari basis data, rotasi<br>tahunan, pencatatan dekripsi, rencana<br>tanggap insiden|
|**.3 Penyelesaian Butir**<br>Seluruh butir yang sebel<br>elah dijawab pada sesi konfir<br>elas hingga tujuh belas. Tab<br>ada desain; rincian teknisny<br>ancangan yang masih mengg<br>**abel 32. Penyelesaian butir konfi**<br>**Butir Konfirmasi**|**Konfirmasi**<br>umnya dinyatakan menunggu<br>masi final dan ditetapkan sebag<br>el berikut merangkum keputusa<br>a telah dipatri ke bab-bab terk<br>antung:<br>**rmasi dan dampaknya pada desain**<br>**Keputusan Final**|keputusan pemilik produk kin<br>ai keputusan desain nomor du<br>n tersebut beserta dampaknya<br>ait sehingga tidak ada bagian<br> <br>**Dampak pada Desain**|
|Lingkup nomor urut NIA|Per kabupaten/kota; setiap wilayah<br>mulai dari 000001 dengan batas<br>999.999 kader|Struktur niaCounter tetap satu baris<br>per kab/kota (Subbab 4.2.3);<br>penomoran antarwilayah saling<br>mandiri|
|Format penomoran SK|Diisi manual oleh admin penyusun;<br>lampiran berkas SK wajib|nomorSK teks bebas namun unik +<br>validasi keberadaan lampiran saat<br>pengajuan (Subbab 4.2.4)|
|Daftar tetap masterJabatan|Dikelola manual; admin dapat<br>menambah jabatan sendiri termasuk<br>menandai isKetuaUmum|CRUD masterJabatan + audit<br>TAMBAH_JABATAN (Subbab 6.8);<br>tanpa daftar bawaan sistem|
|Penyedia notifikasi WhatsApp|Ditunda - gateway belum dipilih pada<br>fase ini|Pelacakan status sepenuhnya via NIK<br>+ tanggal lahir; hook notifikasi siap<br>dipasang kemudian (Subbab 5.1)|
|Kebijakan retensi pendaftar<br>DITOLAK|Data tidak dihapus - disimpan<br>permanen sebagai arsip|Tanpa cron penghapusan; indeks unik<br>nikHash hanya untuk status non-<br>DITOLAK sehingga pendaftar dapat<br>mendaftar ulang (Subbab 4.2.2)|
|Tanda tangan pimpinan pada e-KTA|Citra statis tanda tangan Ketua<br>Umum yang telah tersedia|Aset terproteksi pada generator e-<br>KTA; keaslian via QR ke halaman<br>validasi publik (Subbab 7.4)|



### **8.3 Penyelesaian Butir Konfirmasi** 

Seluruh butir yang sebelumnya dinyatakan menunggu keputusan pemilik produk kini telah dijawab pada sesi konfirmasi final dan ditetapkan sebagai keputusan desain nomor dua belas hingga tujuh belas. Tabel berikut merangkum keputusan tersebut beserta dampaknya pada desain; rincian teknisnya telah dipatri ke bab-bab terkait sehingga tidak ada bagian rancangan yang masih menggantung: 

**Tabel 32. Penyelesaian butir konfirmasi dan dampaknya pada desain** 

38 

Dokumen Desain Teknis SIM KIPAN - v1.1 

Dengan seluruh keputusan desain, struktur data, spesifikasi API, dan algoritma bisnis yang telah terdefinisi pada dokumen ini, tim implementasi dapat memulai Fase 1 tanpa ambiguitas. Setiap perubahan kebutuhan pada masa implementasi disarankan dicatat sebagai tambalan pada log keputusan desain agar dokumen ini tetap menjadi sumber kebenaran tunggal yang hidup sepanjang siklus pembangunan SIM KIPAN. 

39 

