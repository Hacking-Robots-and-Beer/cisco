import Link from "next/link";

export default function Navbar() {
  return (
    <nav className="bg-primary text-white shadow-md">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-14">
          <Link href="/" className="font-bold text-lg tracking-wide hover:opacity-80 transition">
            Cisco AP Controller
          </Link>
          <div className="flex items-center gap-6 text-sm">
            <Link href="/" className="hover:opacity-80 transition">
              Dashboard
            </Link>
            <Link
              href="/aps/new"
              className="bg-white text-primary px-3 py-1 rounded font-semibold hover:bg-blue-50 transition"
            >
              + Add AP
            </Link>
          </div>
        </div>
      </div>
    </nav>
  );
}
