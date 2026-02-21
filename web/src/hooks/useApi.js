import { useState, useCallback } from 'react';
import { useToast } from '../context/ToastContext';

export function useApi() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const { showToast } = useToast();

  const execute = useCallback(async (apiCall, options = {}) => {
    const {
      successMessage,
      errorMessage,
      showSuccessToast = false,
      showErrorToast = true
    } = options;

    setLoading(true);
    setError(null);

    try {
      const result = await apiCall();
      if (showSuccessToast && successMessage) {
        showToast(successMessage, 'success');
      }
      return result;
    } catch (err) {
      setError(err.message);
      if (showErrorToast) {
        showToast(errorMessage || err.message, 'error');
      }
      throw err;
    } finally {
      setLoading(false);
    }
  }, [showToast]);

  return { loading, error, execute };
}
