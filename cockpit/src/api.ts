const ctl='/usr/libexec/cockpit-ups-wol/cockpit-ups-wolctl';
export interface HealthResult{name:string;ok:boolean;critical:boolean;repairable:boolean;message?:string}
export interface HealthSnapshot{state:string;checked_at:string;results:HealthResult[];generation:number;circuit?:{failed_safe_reason?:string}}
export interface HealthResponse{ok:boolean;result?:HealthSnapshot;error?:{message?:string}}
export interface Revision{revision_id:string;created_at:string;status:string;source:string}
export interface ConfigStatus{active:string;last_known_good:string;previous_known_good:string;active_manifest?:Revision;last_known_good_manifest?:Revision;known_good_revisions:Revision[]}
export interface PlanHost{id:string;name:string;priority?:number;method?:string;restore_policy?:string;wake_enabled?:boolean;wake_priority?:number;depends_on?:string[]}
export interface Plan{mode:string;ups:{profile:string;target:string;power_cycle_capability:string;synology_compatibility:boolean};outage:{grace_period_seconds:number;critical_battery_percent?:number;critical_runtime_seconds?:number};recovery:{enabled:boolean;utility_stable_seconds:number;battery_charge_min?:number;runtime_min_seconds?:number;recharge_time_seconds?:number;network_wait_seconds:number};shutdown:PlanHost[];restore:PlanHost[];network_dependencies:Array<{id:string;name:string;startup:string;priority:number;status_method:string}>}
export interface ConfigValidation{valid:boolean;plan:Plan}
async function jsonCommand<T>(args:string[],superuser:'try'|'require'='require'):Promise<T>{const text=await window.cockpit.spawn([ctl,...args],{err:'message',superuser});return JSON.parse(text)as T}
async function inputCommand<T>(args:string[],input:string,superuser:'try'|'require'='require'):Promise<T>{const process=window.cockpit.spawn([ctl,...args],{err:'message',superuser});process.input(input);const text=await process;return JSON.parse(text)as T}
export const getHealth=()=>jsonCommand<HealthResponse>(['health']);
export const getConfigStatus=()=>jsonCommand<ConfigStatus>(['config-status']);
export const getPlan=()=>jsonCommand<Plan>(['plan']);
export const getLogs=()=>window.cockpit.spawn([ctl,'logs'],{err:'message',superuser:'require'});
export const getConfigText=()=>window.cockpit.spawn([ctl,'config-get'],{err:'message',superuser:'require'});
export const validateConfig=(text:string)=>inputCommand<ConfigValidation>(['config-validate'],text);
export const applyConfig=(text:string)=>inputCommand<ConfigStatus>(['config-apply'],text);
export const rollbackConfig=(id:string)=>jsonCommand<ConfigStatus>(['config-rollback',id]);
