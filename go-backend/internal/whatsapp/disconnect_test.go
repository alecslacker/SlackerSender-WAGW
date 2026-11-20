package whatsapp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// newTestManager memakai GORM DryRun + driver postgres tanpa koneksi nyata:
// query tidak dieksekusi sehingga tes berjalan tanpa DB maupun cgo/sqlite.
// Yang diuji di sini adalah logika pembatalan sesi, bukan isi tabel.
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	db, err := gorm.Open(
		postgres.New(postgres.Config{DSN: "host=127.0.0.1 port=1 dbname=x"}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true},
	)
	if err != nil {
		t.Fatalf("open dry-run db: %v", err)
	}
	return &Manager{
		sessions:   make(map[uint]*SessionState),
		sessionDir: t.TempDir(),
		db:         db,
		retryGen:   make(map[uint]int),
		manualStop: make(map[uint]bool),
		presence:   make(map[uint]map[string]PresenceState),
	}
}

// Disconnect saat sesi masih "connecting" (Client belum terpasang) harus
// membatalkan sesi, supaya startSession yang masih berjalan tidak
// menyambungkan ulang perangkat.
func TestDisconnectWhileConnectingCancelsSession(t *testing.T) {
	m := newTestManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	sess := &SessionState{DeviceID: 1, UserID: 1, Status: "connecting", ctx: ctx, cancel: cancel}
	m.sessions[1] = sess

	if sess.cancelled() {
		t.Fatal("sesi baru tidak boleh langsung dianggap dibatalkan")
	}

	m.Disconnect(1)

	if !sess.cancelled() {
		t.Fatal("Disconnect harus membatalkan ctx sesi")
	}
	if got := m.GetStatus(1); got != "disconnected" {
		t.Fatalf("status = %q, want disconnected", got)
	}
	if !m.manualStop[1] {
		t.Fatal("Disconnect harus menandai manualStop")
	}
}

// Retry otomatis (connectLocked) tidak boleh menghapus tanda stop manual;
// hanya aksi pengguna (Connect) yang boleh.
func TestConnectLockedKeepsManualStop(t *testing.T) {
	m := newTestManager(t)
	m.manualStop[2] = true
	m.sessions[2] = &SessionState{DeviceID: 2, UserID: 1, Status: "connecting"}

	m.mu.Lock()
	_ = m.connectLocked(2, 1) // sesi aktif → no-op
	m.mu.Unlock()

	if !m.manualStop[2] {
		t.Fatal("connectLocked tidak boleh menghapus manualStop")
	}
}

func TestConnectClearsManualStopAndBumpsRetryGen(t *testing.T) {
	m := newTestManager(t)
	m.manualStop[3] = true
	gen := m.retryGen[3]
	m.sessions[3] = &SessionState{DeviceID: 3, UserID: 1, Status: "connecting"}

	if err := m.Connect(3, 1); err != nil {
		t.Fatal(err)
	}
	if m.manualStop[3] {
		t.Fatal("Connect dari pengguna harus menghapus manualStop")
	}
	if m.retryGen[3] != gen+1 {
		t.Fatal("Connect harus membatalkan retry loop lama (retryGen naik)")
	}
}

// Logout pada perangkat tanpa sesi aktif: menandai manualStop (tidak di-retry)
// dan menghapus file sesi lokal sehingga Hubungkan berikutnya minta QR.
func TestLogoutRemovesLocalSessionFiles(t *testing.T) {
	m := newTestManager(t)
	// File sesi kosong/bukan SQLite valid: Logout harus tetap membersihkannya.
	base := filepath.Join(m.sessionDir, "device_4.db")
	for _, f := range []string{base, base + "-wal", base + "-shm"} {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	_ = m.Logout(4) // error pencabutan di server boleh; sesi lokal wajib hilang

	if !m.manualStop[4] {
		t.Fatal("Logout harus menandai manualStop agar tidak di-retry")
	}
	for _, f := range []string{base, base + "-wal", base + "-shm"} {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Fatalf("file sesi %s harus terhapus (err=%v)", f, err)
		}
	}
}
