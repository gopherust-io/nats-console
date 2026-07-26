export const ADMIN = {
  username: "admin",
  roles: ["admin"],
  isRoot: true,
  accessRules: {
    manageUsers: true,
    viewAudit: true,
    deleteClusters: true,
  },
};

export const VIEWER = {
  username: "viewer",
  roles: ["viewer"],
  isRoot: false,
  accessRules: {
    manageUsers: false,
    viewAudit: false,
    deleteClusters: false,
    clusterIds: ["cluster-1"],
  },
};

export type AuthUserFixture = typeof ADMIN | typeof VIEWER;
