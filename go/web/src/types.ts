export type PageKind = "home" | "findings" | "docs" | "activate" | "admin-login" | "admin";

export type BoardRow = {
  Rank: number;
  Company: string;
  MarketCap: string;
};

export type ProjectData = {
  title: string;
  heading: string;
  boardRows?: BoardRow[];
  boardTotal?: number;
  markdown?: string;
  serverAddr?: string;
};

export type AdminStats = {
  Submissions: number;
  Accounts: number;
  Events: number;
  LastSubmission: string;
};

export type LeaderboardRow = {
  Company: string;
  CEO: string;
  MarketCents: number;
};

export type AdminSubmission = {
  ID: number;
  When: string;
  Remote: string;
  Host: string;
  Valid: boolean;
  Computed: number;
  Client: number;
  Fields: Record<string, string>;
  Row: LeaderboardRow;
};

export type AccountRequest = {
  ID: number;
  ReceivedAt: string;
  RemoteAddr: string;
  Username: string;
  Password: string;
  RawQuery: string;
};

export type DetailField = {
  Label: string;
  Value: string;
};

export type AdminEvent = {
  ID: number;
  Time: string;
  Kind: string;
  Status: number;
  RemoteAddr: string;
  Route: string;
  Summary?: DetailField[];
  Raw: string;
  Message: string;
};

export type AdminData = {
  Title: string;
  AdminPath: string;
  Active: "submissions" | "accounts" | "events";
  User: string;
  Stats: AdminStats;
  Flash: string;
  CSRF: string;
  Submissions?: AdminSubmission[];
  Accounts?: AccountRequest[];
  Events?: AdminEvent[];
  Fields: string[];
};

export type AdminLoginData = {
  AdminPath: string;
  Error: string;
  User: string;
};
