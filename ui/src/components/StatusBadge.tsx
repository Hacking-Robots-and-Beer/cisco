type Status = "unknown" | "online" | "offline" | "syncing" | "error";

interface Props {
  status: Status;
}

const colours: Record<Status, string> = {
  unknown:  "bg-gray-100 text-gray-600",
  online:   "bg-green-100 text-green-800",
  offline:  "bg-red-100 text-red-700",
  syncing:  "bg-yellow-100 text-yellow-800",
  error:    "bg-orange-100 text-orange-800",
};

export default function StatusBadge({ status }: Props) {
  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${colours[status] ?? colours.unknown}`}>
      {status}
    </span>
  );
}
