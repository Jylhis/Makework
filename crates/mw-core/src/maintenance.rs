use std::path::Path;
use std::process::Command;

use crate::repository::GitError;

pub fn maintenance_register(bare_path: &Path) -> Result<(), GitError> {
    let output = Command::new("git")
        .arg("-C")
        .arg(bare_path)
        .args(["maintenance", "register"])
        .output()
        .map_err(|e| GitError::Command {
            cmd: format!("git -C {} maintenance register", bare_path.display()),
            stderr: e.to_string(),
        })?;

    if !output.status.success() {
        return Err(GitError::Command {
            cmd: format!("git -C {} maintenance register", bare_path.display()),
            stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
        });
    }

    Ok(())
}

pub fn maintenance_unregister(bare_path: &Path) -> Result<(), GitError> {
    let output = Command::new("git")
        .arg("-C")
        .arg(bare_path)
        .args(["maintenance", "unregister"])
        .output()
        .map_err(|e| GitError::Command {
            cmd: format!("git -C {} maintenance unregister", bare_path.display()),
            stderr: e.to_string(),
        })?;

    if !output.status.success() {
        return Err(GitError::Command {
            cmd: format!("git -C {} maintenance unregister", bare_path.display()),
            stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
        });
    }

    Ok(())
}

pub fn maintenance_status(bare_path: &Path) -> Result<bool, GitError> {
    let output = Command::new("git")
        .arg("-C")
        .arg(bare_path)
        .args(["config", "--get", "maintenance.repo"])
        .output()
        .map_err(|e| GitError::Command {
            cmd: format!(
                "git -C {} config --get maintenance.repo",
                bare_path.display()
            ),
            stderr: e.to_string(),
        })?;

    Ok(output.status.success())
}
