{
  defaultPackage ? null,
}:

{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.services.lsx-server;

  boolString = value: if value then "true" else "false";
  stateDirectory = lib.strings.removePrefix "/var/lib/" (toString cfg.dataDir);
  addrPortMatch = builtins.match ".*:([0-9]+)" cfg.addr;
  firewallPort =
    if addrPortMatch == null then 80 else builtins.fromJSON (builtins.elemAt addrPortMatch 0);

  packageOption = {
    type = lib.types.package;
    example = lib.options.literalExpression "inputs.lsx-server.packages.${pkgs.stdenv.hostPlatform.system}.default";
    description = "The LSX Server package to run.";
  }
  // lib.attrsets.optionalAttrs (defaultPackage != null) {
    default = defaultPackage.${pkgs.stdenv.hostPlatform.system}.default;
    defaultText = lib.options.literalExpression "lsx-server.packages.\${pkgs.stdenv.hostPlatform.system}.default";
  };

  environment = {
    LSX_ADDR = cfg.addr;
    LSX_DATA = "${cfg.dataDir}/lsx.sqlite3";
    LSX_SEED = boolString cfg.seed;
    LSX_STRICT_CHECKSUM = boolString cfg.strictChecksum;
    LSX_PLAIN = boolString cfg.plain;
    LSX_DISCORD_EVENTS = lib.strings.concatStringsSep "," cfg.discordEvents;
    LSX_DISCORD_ICON = cfg.discordIcon;
    LSX_DISCORD_TIMEOUT = cfg.discordTimeout;
  }
  // lib.attrsets.optionalAttrs (cfg.adminUser != null) { LSX_ADMIN_USER = cfg.adminUser; }
  // lib.attrsets.optionalAttrs (cfg.adminPath != null) { LSX_ADMIN_PATH = cfg.adminPath; }
  // cfg.extraEnvironment;

  bindCapability = lib.lists.optional cfg.grantBindServiceCapability "CAP_NET_BIND_SERVICE";

  startScript = pkgs.writeShellScript "lsx-server-start" ''
    set -eu
    ${lib.strings.optionalString (cfg.adminPasswordFile != null) ''
      export LSX_ADMIN_PASSWORD="$(cat "$CREDENTIALS_DIRECTORY/admin-password")"
    ''}
    ${lib.strings.optionalString (cfg.discordWebhookFile != null) ''
      export LSX_DISCORD_WEBHOOK="$(cat "$CREDENTIALS_DIRECTORY/discord-webhook")"
    ''}
    exec ${lib.meta.getExe cfg.package}
  '';
in
{
  options.services.lsx-server = {
    enable = lib.options.mkEnableOption "the Lemonade Tycoon 2 LSX compatibility server";

    package = lib.options.mkOption packageOption;

    addr = lib.options.mkOption {
      type = lib.types.str;
      default = ":80";
      example = "127.0.0.1:8080";
      description = "Address the LSX HTTP server binds to.";
    };

    dataDir = lib.options.mkOption {
      type = lib.types.path;
      default = "/var/lib/lsx-server";
      description = "State directory for the SQLite database. Must live below `/var/lib`.";
    };

    seed = lib.options.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Seed recovered Wayback leaderboard rows on startup.";
    };

    strictChecksum = lib.options.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Reject score uploads whose recovered client checksum does not match.";
    };

    plain = lib.options.mkOption {
      type = lib.types.bool;
      default = true;
      description = "Use plain request logs instead of the interactive terminal UI.";
    };

    adminUser = lib.options.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      example = "admin";
      description = "Admin console username. Set `adminPasswordFile` as well to enable admin login.";
    };

    adminPasswordFile = lib.options.mkOption {
      type = lib.types.nullOr lib.types.path;
      default = null;
      description = "File containing the admin password, passed with systemd `LoadCredential`.";
    };

    adminPath = lib.options.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      example = "/back-office";
      description = "Custom admin console URL path.";
    };

    discordWebhookFile = lib.options.mkOption {
      type = lib.types.nullOr lib.types.path;
      default = null;
      description = "File containing the Discord webhook URL, passed with systemd `LoadCredential`.";
    };

    discordEvents = lib.options.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [
        "sync"
        "sync_rejected"
        "sync_error"
        "account"
        "account_error"
      ];
      description = "Discord event kinds to send.";
    };

    discordIcon = lib.options.mkOption {
      type = lib.types.str;
      default = "embedded";
      description = "Discord embed thumbnail source: `embedded`, a file path, or an empty string.";
    };

    discordTimeout = lib.options.mkOption {
      type = lib.types.str;
      default = "5s";
      example = "10s";
      description = "Timeout for each Discord webhook POST.";
    };

    environmentFile = lib.options.mkOption {
      type = lib.types.nullOr lib.types.path;
      default = null;
      description = "Environment file loaded by systemd for additional `LSX_*` settings.";
    };

    extraEnvironment = lib.options.mkOption {
      type = lib.types.attrsOf lib.types.str;
      default = { };
      example = {
        LSX_VERSION = "git-abcdef0";
      };
      description = "Additional environment variables for the service.";
    };

    openFirewall = lib.options.mkEnableOption "opening the configured TCP port in the firewall";

    grantBindServiceCapability = lib.options.mkOption {
      type = lib.types.bool;
      default = true;
      description = "Grant `CAP_NET_BIND_SERVICE` so the service can bind the default port 80.";
    };
  };

  config = lib.modules.mkIf cfg.enable {
    assertions = [
      {
        assertion = lib.strings.hasPrefix "/var/lib/" (toString cfg.dataDir);
        message = "services.lsx-server.dataDir must be below /var/lib so systemd StateDirectory can manage it.";
      }
    ];

    networking.firewall.allowedTCPPorts = lib.modules.mkIf cfg.openFirewall [ firewallPort ];

    systemd.services.lsx-server = {
      description = "Lemonade Tycoon 2 LSX compatibility server";
      wantedBy = [ "multi-user.target" ];
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];

      inherit environment;

      serviceConfig = {
        Type = "exec";
        ExecStart = startScript;
        Restart = "on-failure";
        RestartSec = "5s";
        WorkingDirectory = cfg.dataDir;
        EnvironmentFile = lib.modules.mkIf (cfg.environmentFile != null) [ cfg.environmentFile ];
        LoadCredential =
          lib.lists.optional (cfg.adminPasswordFile != null) "admin-password:${cfg.adminPasswordFile}"
          ++ lib.lists.optional (cfg.discordWebhookFile != null) "discord-webhook:${cfg.discordWebhookFile}";

        DynamicUser = true;
        StateDirectory = stateDirectory;
        StateDirectoryMode = "0750";
        RuntimeDirectory = "lsx-server";
        RuntimeDirectoryMode = "0750";
        UMask = "0077";

        AmbientCapabilities = bindCapability;
        CapabilityBoundingSet = bindCapability;
        LockPersonality = true;
        MemoryDenyWriteExecute = true;
        NoNewPrivileges = true;
        PrivateDevices = true;
        PrivateMounts = true;
        PrivateTmp = true;
        ProcSubset = "pid";
        ProtectClock = true;
        ProtectControlGroups = true;
        ProtectHome = true;
        ProtectHostname = true;
        ProtectKernelLogs = true;
        ProtectKernelModules = true;
        ProtectKernelTunables = true;
        ProtectProc = "invisible";
        ProtectSystem = "strict";
        RemoveIPC = true;
        RestrictAddressFamilies = [
          "AF_INET"
          "AF_INET6"
          "AF_UNIX"
        ];
        RestrictNamespaces = true;
        RestrictRealtime = true;
        RestrictSUIDSGID = true;
        SystemCallArchitectures = "native";
        SystemCallFilter = [
          "@system-service"
          "~@privileged"
          "~@resources"
        ];
        ExecPaths = [ "/nix/store" ];
        NoExecPaths = [ "/" ];
      };
    };
  };
}
