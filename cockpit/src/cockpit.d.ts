export {};

declare global {
  interface CockpitSpawnOptions {
    err?: "message" | "out";
    superuser?: "try" | "require";
  }
  interface CockpitProcess extends Promise<string> {
    input(data: string | null | undefined, stream?: boolean): void;
  }
  interface CockpitAPI {
    spawn(command: string[], options?: CockpitSpawnOptions): CockpitProcess;
  }
  interface Window {
    cockpit: CockpitAPI;
  }
}
