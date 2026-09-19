export {};

declare global {
  interface CockpitSpawnOptions { err?: 'message' | 'out'; superuser?: 'try' | 'require'; }
  interface CockpitAPI { spawn(command: string[], options?: CockpitSpawnOptions): Promise<string>; }
  interface Window { cockpit: CockpitAPI; }
}
