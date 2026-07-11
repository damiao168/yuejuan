ALTER TABLE capture_operation
  DROP CONSTRAINT capture_operation_operation_check;

ALTER TABLE capture_operation
  ADD CONSTRAINT capture_operation_operation_check
  CHECK (operation IN (
    'rotate', 'reorder', 'split', 'merge', 'delete', 'restore',
    'student_match', 'page_match', 'override', 'reopen', 'cancel', 'complete'
  ));
