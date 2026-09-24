import UserListPage from "@/modules/users/pages/UserListPage";
import { MySignaturePanel } from "@/modules/users/components/MySignaturePanel";
import { useAuthStore } from "@/shared/stores/useAuthStore";

// Rute /pengguna untuk semua role staff. Owner melihat manajemen pengguna
// penuh (TTD dirinya sendiri diurus lewat modal edit barisnya); Admin/Staff/
// Sales hanya melihat TTD miliknya sendiri — satu-satunya bagian menu Pengguna
// yang boleh mereka sentuh, karena seluruh manajemen pengguna lain Owner-only
// di backend (staff_handler.go requireOwnerTenant).
//
// Dicabang di komponen terpisah, bukan di dalam UserListPage: UserListPage
// memuat daftar pengguna saat mount (akan 403 untuk non-Owner), dan hook-nya
// tidak boleh dijalankan secara kondisional.
export default function UsersPage() {
  const role = useAuthStore((s) => s.session?.role);

  if (role === "Owner") {
    return <UserListPage />;
  }

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-bold text-text-primary">Pengguna</h1>
        <p className="mt-1 text-[13px] text-text-secondary">Kelola tanda tangan akun Anda sendiri.</p>
      </div>
      <MySignaturePanel />
    </div>
  );
}
