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

  environmentList = lib.attrsets.mapAttrsToList (name: value: "${name}=${value}") environment;

  startScript = pkgs.writeShellScript "lsx-server-start" ''
    set -eu
    mkdir -p ${lib.strings.escapeShellArg (toString cfg.dataDir)}
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
      example = ":8080";
      description = "Address the LSX HTTP server binds to.";
    };

    dataDir = lib.options.mkOption {
      type = lib.types.path;
      default = "${config.xdg.dataHome}/lsx-server";
      description = "State directory for the SQLite database.";
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

  config = lib.modules.mkIf cfg.enable (
    lib.modules.mkMerge [
      {
        home.packages = [ cfg.package ];
      }

      (lib.modules.mkIf pkgs.stdenv.isLinux {
        systemd.user.services.lsx-server = {
          Unit = {
            Description = "Lemonade Tycoon 2 LSX compatibility server";
            After = [ "network.target" ];
          };

          Service = {
            ExecStart = startScript;
            Environment = environmentList;
            Restart = "on-failure";
            RestartSec = "5s";
            WorkingDirectory = cfg.dataDir;
            UMask = "0077";
          };

          Install.WantedBy = [ "default.target" ];
        };
      })

      (lib.modules.mkIf pkgs.stdenv.isDarwin {
        launchd.agents.lsx-server = {
          enable = true;
          config = {
            ProgramArguments = [ startScript ];
            EnvironmentVariables = environment;
            KeepAlive = {
              Crashed = true;
              SuccessfulExit = false;
            };
            ProcessType = "Background";
            RunAtLoad = true;
            WorkingDirectory = cfg.dataDir;
            StandardOutPath = "${cfg.dataDir}/lsx-server.log";
            StandardErrorPath = "${cfg.dataDir}/lsx-server.log";
          };
        };
      })
    ]
  );
}
