// Package auth menangani identitas: login JWT + refresh, ganti password,
// dan reset kata sandi mandiri via email. Boleh mengimpor: config, domain,
// gateway, repository, mail, svcutil (+ pustaka jwt/redis/argon2id).
// Tidak boleh mengimpor subpackage service lain.
package auth
