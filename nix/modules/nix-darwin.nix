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
  serviceUser = config.users.users.${cfg.userName};

  boolString = value: if value then "true" else "false";

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

  startScript = pkgs.writeShellScript "lsx-server-start" ''
    set -eu
    ${lib.strings.optionalString (cfg.adminPasswordFile != null) ''
      export LSX_ADMIN_PASSWORD="$(cat ${lib.strings.escapeShellArg (toString cfg.adminPasswordFile)})"
    ''}
    ${lib.strings.optionalString (cfg.discordWebhookFile != null) ''
      export LSX_DISCORD_WEBHOOK="$(cat ${lib.strings.escapeShellArg (toString cfg.discordWebhookFile)})"
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
      default = "127.0.0.1:8080";
      example = ":80";
      description = "Address the LSX HTTP server binds to.";
    };

    dataDir = lib.options.mkOption {
      type = lib.types.path;
      default = "/var/lib/lsx-server";
      description = "State directory for the SQLite database.";
    };

    logFile = lib.options.mkOption {
      type = lib.types.path;
      default = "/var/log/lsx-server.log";
      description = "Log file used for launchd stdout and stderr.";
    };

    userName = lib.options.mkOption {
      type = lib.types.str;
      default = "_lsx-server";
      description = "User that runs the launchd daemon.";
    };

    groupName = lib.options.mkOption {
      type = lib.types.str;
      default = "_lsx-server";
      description = "Group that runs the launchd daemon.";
    };

    uid = lib.options.mkOption {
      type = lib.types.int;
      default = 532;
      description = "UID for the LSX Server service user.";
    };

    gid = lib.options.mkOption {
      type = lib.types.int;
      default = 532;
      description = "GID for the LSX Server service group.";
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
      description = "File containing the admin password.";
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
      description = "File containing the Discord webhook URL.";
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

    extraEnvironment = lib.options.mkOption {
      type = lib.types.attrsOf lib.types.str;
      default = { };
      example = {
        LSX_VERSION = "git-abcdef0";
      };
      description = "Additional environment variables for the service.";
    };
  };

  config = lib.modules.mkIf cfg.enable {
    environment.systemPackages = [ cfg.package ];

    launchd.daemons.lsx-server = {
      serviceConfig = {
        ProgramArguments = [ startScript ];
        EnvironmentVariables = environment;
        KeepAlive = true;
        RunAtLoad = true;
        StandardErrorPath = cfg.logFile;
        StandardOutPath = cfg.logFile;
        UserName = cfg.userName;
        GroupName = cfg.groupName;
        WorkingDirectory = cfg.dataDir;
      };
    };

    users.knownUsers = [ cfg.userName ];
    users.knownGroups = [ cfg.groupName ];
    users.users.${cfg.userName} = {
      uid = lib.modules.mkDefault cfg.uid;
      gid = lib.modules.mkDefault config.users.groups.${cfg.groupName}.gid;
      home = lib.modules.mkDefault cfg.dataDir;
      name = cfg.userName;
      createHome = true;
      shell = "/usr/bin/false";
      description = "LSX Server service user";
    };
    users.groups.${cfg.groupName} = {
      gid = lib.modules.mkDefault cfg.gid;
      name = cfg.groupName;
      description = "LSX Server service group";
    };

    system.activationScripts.preActivation.text = ''
      mkdir -p '${cfg.dataDir}' "$(dirname '${cfg.logFile}')"
      touch '${cfg.logFile}'
      chown -R ${serviceUser.name}:${cfg.groupName} '${cfg.dataDir}'
      chown ${serviceUser.name}:${cfg.groupName} '${cfg.logFile}'
      chmod 0750 '${cfg.dataDir}'
      chmod 0640 '${cfg.logFile}'
    '';
  };
}
