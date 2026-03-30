;;; makework-test.el --- Tests for makework.el -*- lexical-binding: t -*-

;;; Commentary:

;; ERT tests for makework Emacs integration.

;;; Code:

(require 'ert)
(require 'makework)

(ert-deftest makework-test-parse-go-output ()
  "Test parsing mw go output with nix info."
  (let ((parsed (makework--parse-go-output
                 "/path/to/worktree\nnix develop\n/path/to/nix")))
    (should (equal (cdr (assq 'path parsed)) "/path/to/worktree"))
    (should (equal (cdr (assq 'nix-cmd parsed)) "nix develop"))
    (should (equal (cdr (assq 'nix-dir parsed)) "/path/to/nix"))))

(ert-deftest makework-test-parse-go-output-no-nix ()
  "Test parsing mw go output without nix info."
  (let ((parsed (makework--parse-go-output "/path/to/worktree")))
    (should (equal (cdr (assq 'path parsed)) "/path/to/worktree"))
    (should (null (cdr (assq 'nix-cmd parsed))))
    (should (null (cdr (assq 'nix-dir parsed))))))

(ert-deftest makework-test-build-command ()
  "Test command building."
  (should (equal (makework--build-command "go" "myproject")
                 '("mw" "go" "myproject")))
  ;; Test with custom binary
  (let ((makework-binary "/usr/local/bin/mw"))
    (should (equal (makework--build-command "go" "myproject")
                   '("/usr/local/bin/mw" "go" "myproject")))))

(ert-deftest makework-test-parse-status-output ()
  "Test parsing status output."
  (let ((parsed (makework--parse-status-output
                 "repo-a:\n  main  /path/a\n  feature/x  /path/b\nrepo-b:\n  main  /path/c\n")))
    (should (= (length parsed) 2))
    (should (equal (car (nth 0 parsed)) "repo-a"))
    (should (= (length (cdr (nth 0 parsed))) 2))
    (should (equal (car (nth 1 parsed)) "repo-b"))
    (should (= (length (cdr (nth 1 parsed))) 1))))

(provide 'makework-test)

;;; makework-test.el ends here
