;;; makework.el --- Emacs integration for makework worktree manager -*- lexical-binding: t -*-

;; Copyright (C) 2026
;; Author: Makework Contributors
;; Version: 0.1.0
;; Package-Requires: ((emacs "28.1"))
;; Keywords: tools, vc
;; URL: https://github.com/jylhis/Makework

;;; Commentary:

;; Emacs integration for the makework git worktree manager.
;; Provides commands for navigating projects, viewing status,
;; syncing repos, and fetching updates.

;;; Code:

(defgroup makework nil
  "Emacs integration for makework worktree manager."
  :group 'tools
  :prefix "makework-")

(defcustom makework-binary "mw"
  "Path to the makework binary."
  :type 'string
  :group 'makework)

(defcustom makework-use-nix t
  "Whether to activate nix environment when navigating to projects."
  :type 'boolean
  :group 'makework)

(defun makework--build-command (&rest args)
  "Build a makework command from ARGS."
  (cons makework-binary args))

(defun makework--parse-go-output (output)
  "Parse OUTPUT from `mw go` into an alist.
Returns an alist with keys `path', `nix-cmd', and `nix-dir'."
  (let* ((lines (split-string output "\n" t))
         (path (nth 0 lines))
         (nix-cmd (nth 1 lines))
         (nix-dir (nth 2 lines)))
    `((path . ,path)
      (nix-cmd . ,(if (and nix-cmd (not (string-empty-p nix-cmd))) nix-cmd nil))
      (nix-dir . ,(if (and nix-dir (not (string-empty-p nix-dir))) nix-dir nil)))))

(defun makework--parse-status-output (output)
  "Parse OUTPUT from `mw` (status) into a structured list.
Returns a list of (repo-name . ((branch . status-line) ...))."
  (let ((lines (split-string output "\n" t))
        (result '())
        (current-repo nil))
    (dolist (line lines)
      (cond
       ;; Repo name line (no leading whitespace, ends with :)
       ((string-match "^\\([^ ].*\\):$" line)
        (setq current-repo (match-string 1 line))
        (push (cons current-repo '()) result))
       ;; Worktree line (leading whitespace)
       ((and current-repo (string-match "^ " line))
        (let ((entry (string-trim line)))
          (when (not (string-empty-p entry))
            (let ((repo-entry (assoc current-repo result)))
              (when repo-entry
                (setcdr repo-entry (append (cdr repo-entry) (list entry))))))))))
    (nreverse result)))

(defun makework--project-list ()
  "Return a list of all project names from the catalog."
  (let ((output (shell-command-to-string
                 (mapconcat #'shell-quote-argument
                            (makework--build-command "catalog" "list")
                            " "))))
    (let ((lines (cdr (split-string output "\n" t)))
          (names '()))
      (dolist (line lines)
        (when (string-match "^\\([^ ]+\\)" line)
          (push (match-string 1 line) names)))
      (nreverse names))))

;;;###autoload
(defun makework-go ()
  "Navigate to a makework project using completing-read."
  (interactive)
  (let* ((projects (makework--project-list))
         (project (completing-read "Project: " projects nil t))
         (cmd (mapconcat #'shell-quote-argument
                         (makework--build-command "go" project)
                         " "))
         (output (shell-command-to-string cmd))
         (parsed (makework--parse-go-output output))
         (path (cdr (assq 'path parsed)))
         (nix-cmd (cdr (assq 'nix-cmd parsed)))
         (nix-dir (cdr (assq 'nix-dir parsed))))
    (when path
      (cd path)
      (message "Changed to %s" path)
      (when (and makework-use-nix nix-cmd)
        (let ((default-directory (or nix-dir path)))
          (compile nix-cmd))))))

;;;###autoload
(defun makework-status ()
  "Display makework status in a buffer."
  (interactive)
  (let* ((cmd (mapconcat #'shell-quote-argument
                         (makework--build-command)
                         " "))
         (output (shell-command-to-string cmd)))
    (with-current-buffer (get-buffer-create "*makework-status*")
      (let ((inhibit-read-only t))
        (erase-buffer)
        (insert output))
      (special-mode)
      (goto-char (point-min))
      (display-buffer (current-buffer)))))

;;;###autoload
(defun makework-sync ()
  "Run `mw sync' and display results."
  (interactive)
  (let* ((cmd (mapconcat #'shell-quote-argument
                         (makework--build-command "sync")
                         " "))
         (output (shell-command-to-string cmd)))
    (message "%s" (string-trim output))))

;;;###autoload
(defun makework-fetch ()
  "Run `mw fetch' in a compilation buffer."
  (interactive)
  (compile (mapconcat #'shell-quote-argument
                      (makework--build-command "fetch")
                      " ")))

(provide 'makework)

;;; makework.el ends here
