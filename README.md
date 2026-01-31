# Btrfs WebUI Manager for NixOS

> **Note:** This project is entirely generated and maintained by **Gemini CLI** under human guidance.

![WebUI Screenshot](./screenshot-webui.png)

A lightweight, Go-based web interface designed specifically for managing Btrfs snapshots on NixOS via **Snapper**. This project aims to provide a clean, "File Explorer" style dashboard for users who want to visualize their snapshot history and perform granular file restorations without leaving the browser.

## 🚀 Features

*   **Snapshot Browser:** List all snapshots for different Snapper configurations (e.g., `home`, `var`).
*   **Diff Explorer:** A tree-view file diff that allows you to see exactly what changed in each snapshot.
*   **Recursive UndoChange:** Restore specific files or entire directories to a previous state with a single click.
*   **Snapshot Management:** Create new manual snapshots with descriptions and custom userdata, or delete old ones.
*   **System Awareness:** Real-time monitoring of Snapper config settings, cleanup schedules, and retention policies.

## 🛠 Why No "Rollback"?

Unlike `snapper rollback`, this tool focuses on `undochange`. On **NixOS**, system-level rollbacks are traditionally handled via the bootloader generations. 

Performing a Btrfs-level `rollback` (changing the default subvolume) on NixOS can lead to inconsistencies with `fstab` and the Nix store. To keep your system safe and "Nix-idiomatic," we recommend:
1.  Using **Nix Generations** for full system rollbacks.
2.  Using this **WebUI (UndoChange)** for granular data and configuration file recovery.

## 📦 Installation (NixOS Flake)

Add this repo to your `flake.nix` inputs:

```nix
{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-25.11";
    btrfs-webui.url = "github:anton1615/Btrfs-Webui-for-Nixos";
    # Optional: ensure it uses your system's nixpkgs version
    btrfs-webui.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = { self, nixpkgs, btrfs-webui, ... }@inputs: {
    nixosConfigurations.your-hostname = nixpkgs.lib.nixosSystem {
      specialArgs = { inherit inputs; };
      modules = [
        # ... your other modules
        ({ config, pkgs, inputs, ... }: {
          environment.systemPackages = [ 
            inputs.btrfs-webui.packages.${pkgs.system}.default 
          ];
          
          systemd.services.btrfs-webui = {
            description = "Btrfs Snapshot Web Dashboard";
            after = [ "network.target" ];
            wantedBy = [ "multi-user.target" ];
            serviceConfig = {
              ExecStart = "${inputs.btrfs-webui.packages.${pkgs.system}.default}/bin/Btrfs-Webui-for-Nixos";
              User = "root"; # Required to run snapper commands
              Restart = "always";
            };
          };
        })
      ];
    };
  };
}
```

## ⚙️ Configuration

This UI **does not** manage Snapper configurations (creating new configs or changing intervals). You should define your Snapper configs in your NixOS configuration:

```nix
services.snapper.configs = {
  home = {
    SUBVOLUME = "/home";
    TIMELINE_CREATE = true;
    TIMELINE_CLEANUP = true;
  };
};
```

## 🧪 Technical Details

*   **Backend:** Go (Standard Library only, zero dependencies for core logic).
*   **Frontend:** Pure HTML/JS/CSS (Vanilla, no frameworks, encapsulated via Go `embed`).
*   **Process:** Calls `/run/current-system/sw/bin/snapper` directly to ensure NixOS compatibility.
*   **Testing Environment:** 
    *   **OS:** NixOS 25.11 (Unstable/Stable)
    *   **Architecture:** x86_64
    *   **Hardware:** ASUS X550VC (4GB RAM, i5-3230M)
    *   **Btrfs Layout:** Disko-managed subvolumes.

## 📄 License

This project is licensed under the **MIT License**.

---
*Developed with ❤️ using Gemini CLI.*
