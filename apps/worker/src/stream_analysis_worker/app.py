import json
from dataclasses import asdict, dataclass


@dataclass(frozen=True, slots=True)
class WorkerStatus:
    component: str = "collection-worker"
    status: str = "ready"


def build_startup_message() -> str:
    return json.dumps(asdict(WorkerStatus()), separators=(",", ":"), sort_keys=True)


def main() -> None:
    print(build_startup_message())
