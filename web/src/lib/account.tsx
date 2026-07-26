import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";
import { useCluster } from "./cluster";

const ACCOUNT_STORAGE_KEY = "nats-consol-account";

export type AccountRecord = {
  id: string;
  clusterId: string;
  name: string;
};

type AccountContextValue = {
  accounts: AccountRecord[];
  accountName: string;
  setAccountName: (name: string) => void;
  loading: boolean;
  reload: () => Promise<void>;
};

const AccountContext = createContext<AccountContextValue | null>(null);

function storedAccountKey(clusterId: string) {
  return `${ACCOUNT_STORAGE_KEY}:${clusterId}`;
}

const DEFAULT_ACCOUNT: AccountRecord = {
  id: "default",
  clusterId: "",
  name: "Default",
};

export function AccountProvider({ children }: { children: ReactNode }) {
  const { clusterId } = useCluster();
  const [accountName, setAccountNameState] = useState("Default");

  const accounts = useMemo(
    () => [{ ...DEFAULT_ACCOUNT, clusterId: clusterId ?? "" }],
    [clusterId],
  );

  const setAccountName = useCallback(
    (name: string) => {
      setAccountNameState(name);
      if (clusterId) localStorage.setItem(storedAccountKey(clusterId), name);
    },
    [clusterId],
  );

  const reload = useCallback(async () => {}, []);

  const value = useMemo(
    () => ({
      accounts,
      accountName,
      setAccountName,
      loading: false,
      reload,
    }),
    [accounts, accountName, setAccountName, reload],
  );

  return <AccountContext.Provider value={value}>{children}</AccountContext.Provider>;
}

export function useAccount() {
  const ctx = useContext(AccountContext);
  if (!ctx) throw new Error("useAccount must be used within AccountProvider");
  return ctx;
}
