/**
 * 勤怠 (attendance) — 200 operations. Shares "employee" (社員) with expense
 * and "approval" (経費承認/発注承認/勤怠承認) with expense and purchasing.
 * `OvertimeThreshold` is the axis E example worked through in
 * docs/specs/narrowing.md section 4.
 */
import { ALL_VERBS, pluralOf } from "./naming.ts";
import type { Aggregate, Resource, ServiceFixture, Setting, Workflow } from "./types.ts";

function r(id: string, noun: string, opts?: { group?: string; shared?: string }): Resource {
  return {
    id,
    plural: pluralOf(id),
    noun,
    verbs: ALL_VERBS,
    ...(opts?.group === undefined ? {} : { group: opts.group }),
    ...(opts?.shared === undefined ? {} : { shared: opts.shared }),
  };
}

const resources: readonly Resource[] = [
  // axis C group "time"
  r("AttendanceRecord", "勤怠記録", { group: "time" }),
  r("ClockEvent", "打刻", { group: "time" }),
  r("Overtime", "残業", { group: "time" }),
  r("Absence", "欠勤", { group: "time" }),
  r("Lateness", "遅刻早退", { group: "time" }),
  // axis C group "leave"
  r("PaidLeave", "有給休暇", { group: "leave" }),
  r("LeaveRequest", "休暇申請", { group: "leave" }),
  r("LeaveBalance", "休暇残日数", { group: "leave" }),
  r("SpecialLeave", "特別休暇", { group: "leave" }),
  r("Holiday", "休日", { group: "leave" }),
  // axis C group "org"
  r("Employee", "社員", { group: "org", shared: "employee" }),
  r("Department", "部署", { group: "org" }),
  r("Shift", "シフト", { group: "org" }),
  r("WorkSchedule", "勤務スケジュール", { group: "org" }),
  // ungrouped
  r("Approval", "勤怠承認", { shared: "approval" }),
  r("Timesheet", "出勤簿"),
  r("BusinessTrip", "出張"),
  r("Commute", "通勤経路"),
  r("FlexTime", "フレックス勤務"),
  r("Telework", "テレワーク"),
  r("Break", "休憩"),
  r("WorkLocation", "勤務地"),
  r("Payroll", "給与連携"),
  r("SocialInsurance", "社会保険"),
  r("HealthCheckup", "健康診断"),
  r("AnnualReview", "人事考課"),
  r("Training", "研修受講"),
  r("Qualification", "資格"),
  r("EmergencyContact", "緊急連絡先"),
  r("Grievance", "苦情申立"),
];

const aggregates: readonly Aggregate[] = [
  { id: "AttendanceRecords", kind: "search", noun: "勤怠記録" },
  { id: "ClockEvents", kind: "search", noun: "打刻" },
  { id: "Overtimes", kind: "summarize", noun: "残業" },
  { id: "Absences", kind: "summarize", noun: "欠勤" },
  { id: "Latenesses", kind: "summarize", noun: "遅刻早退" },
  { id: "PaidLeaves", kind: "search", noun: "有給休暇" },
  { id: "LeaveRequests", kind: "search", noun: "休暇申請" },
  { id: "LeaveBalances", kind: "aggregate", noun: "休暇残日数" },
  { id: "SpecialLeaves", kind: "summarize", noun: "特別休暇" },
  { id: "Employees", kind: "search", noun: "社員" },
  { id: "Departments", kind: "search", noun: "部署" },
  { id: "Shifts", kind: "search", noun: "シフト" },
  { id: "WorkSchedules", kind: "summarize", noun: "勤務スケジュール" },
  { id: "BusinessTrips", kind: "summarize", noun: "出張" },
  { id: "Teleworks", kind: "summarize", noun: "テレワーク" },
  { id: "Breaks", kind: "aggregate", noun: "休憩" },
  { id: "HealthCheckups", kind: "search", noun: "健康診断" },
  { id: "Trainings", kind: "summarize", noun: "研修受講" },
  { id: "Qualifications", kind: "search", noun: "資格" },
  { id: "Grievances", kind: "search", noun: "苦情申立" },
];

const settings: readonly Setting[] = [
  // the axis E worked example (docs/specs/narrowing.md section 4).
  {
    id: "OvertimeThreshold",
    verb: "get",
    summary: "残業時間の上限設定を取得",
    displayName: "残業時間上限設定",
  },
  {
    id: "LatenessGraceMinutesSetting",
    verb: "update",
    summary: "遅刻許容時間の設定を更新",
    displayName: "遅刻許容時間設定",
  },
  { id: "LeaveCategory", verb: "list", summary: "休暇区分の一覧", displayName: "休暇区分一覧" },
  {
    id: "PaidLeaveGrantRuleSetting",
    verb: "get",
    summary: "有給休暇付与ルールの設定を取得",
    displayName: "有給休暇付与ルール設定",
  },
  {
    id: "FlexTimeCoreHoursSetting",
    verb: "update",
    summary: "フレックスコアタイムの設定を更新",
    displayName: "コアタイム設定",
  },
  { id: "ShiftCategory", verb: "list", summary: "シフト区分の一覧", displayName: "シフト区分一覧" },
  {
    id: "AbsenceNotificationThreshold",
    verb: "get",
    summary: "欠勤連絡の期限設定を取得",
    displayName: "欠勤連絡期限設定",
  },
  {
    id: "HolidayCalendarSetting",
    verb: "update",
    summary: "休日カレンダーの設定を更新",
    displayName: "休日カレンダー設定",
  },
  {
    id: "DepartmentCategory",
    verb: "list",
    summary: "部署区分の一覧",
    displayName: "部署区分一覧",
  },
  {
    id: "CommuteAllowanceRateSetting",
    verb: "get",
    summary: "通勤手当率の設定を取得",
    displayName: "通勤手当率設定",
  },
  {
    id: "TeleworkApprovalThreshold",
    verb: "update",
    summary: "テレワーク承認の日数上限を更新",
    displayName: "テレワーク承認上限設定",
  },
  {
    id: "QualificationCategory",
    verb: "list",
    summary: "資格区分の一覧",
    displayName: "資格区分一覧",
  },
  {
    id: "BreakDurationDefaultSetting",
    verb: "get",
    summary: "休憩時間の既定値設定を取得",
    displayName: "休憩時間既定値設定",
  },
  {
    id: "BusinessTripApprovalLimitSetting",
    verb: "update",
    summary: "出張承認の金額上限を更新",
    displayName: "出張承認上限設定",
  },
  { id: "TrainingCategory", verb: "list", summary: "研修区分の一覧", displayName: "研修区分一覧" },
  {
    id: "HealthCheckupIntervalSetting",
    verb: "get",
    summary: "健康診断の実施間隔設定を取得",
    displayName: "健康診断間隔設定",
  },
  {
    id: "SpecialLeaveDefaultDaysSetting",
    verb: "update",
    summary: "特別休暇の既定日数を更新",
    displayName: "特別休暇既定日数設定",
  },
  {
    id: "WorkLocationCategory",
    verb: "list",
    summary: "勤務地区分の一覧",
    displayName: "勤務地区分一覧",
  },
  {
    id: "AnnualReviewCycleSetting",
    verb: "get",
    summary: "人事考課サイクルの設定を取得",
    displayName: "人事考課サイクル設定",
  },
  {
    id: "EmergencyContactRequiredSetting",
    verb: "update",
    summary: "緊急連絡先の必須設定を更新",
    displayName: "緊急連絡先必須設定",
  },
];

const workflows: readonly Workflow[] = [
  { id: "LeaveRequest", noun: "休暇申請", actions: ["submit", "approve", "reject", "withdraw"] },
  { id: "Overtime", noun: "残業", actions: ["submit", "approve", "reject", "withdraw"] },
  { id: "BusinessTrip", noun: "出張", actions: ["submit", "approve"] },
];

export const attendance: ServiceFixture = {
  name: "attendance",
  displayName: "勤怠管理",
  resources,
  aggregates,
  settings,
  workflows,
};
