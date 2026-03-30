import React, { useState } from 'react';

interface UserReport {

}

type PageState = 'idle' | 'loading' | 'success' | 'error' | 'not-found' | 'unauthenticated';

const ReportPage: React.FC = () => {
  const [state, setState] = useState<PageState>('idle');
  const [error, setError] = useState<string | null>(null);
  const [report, setReport] = useState<UserReport | null>(null);  

  const handleLogin = () => {
      window.location.href = '/auth/login';
  };

  const handleLogout = async () => {
      const form = document.createElement('form');
      form.method = 'POST';
      form.action = '/auth/logout';
      document.body.appendChild(form);
      form.submit();
  };

  const downloadReport = async () => {
    try {
      setState('loading');
      setError(null);

      const response = await fetch(`/api/reports`, {
        credentials: 'include',
      });

      if (response.ok) {
          const data: UserReport = await response.json();
          setReport(data);
          setState('success');
      } else if (response.status === 401) {
          setState('unauthenticated');
      } else if (response.status === 404) {
          setState('not-found');
      } else {
          const body = await response.json().catch(() => ({}));
          setError(body.error || `Unknown Error (${response.status})`);
          setState('error');
      }
    } catch (err) {
        setError(err instanceof Error ? err.message : 'An error occurred');
        setState('error');        
    }
  };

  return (
    <div className="flex flex-col items-center justify-center min-h-screen bg-gray-100">
      <div className="p-8 bg-white rounded-lg shadow-md">
        <h1 className="text-2xl font-bold mb-6">Usage Reports</h1>       
        <div className="flex gap-3 mb-8">

            <button
                onClick={downloadReport}
                disabled={state === 'loading'}
                className={`px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 ${
                    state === 'loading' ? 'opacity-50 cursor-not-allowed' : ''
                }`}
                >
                    {state === 'loading' ? 'Generating Report...' : 'Download Report'}
            </button>

            <button
                onClick={handleLogout}
                className={`px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600`}
            >
                Logout
            </button>
        </div>

        {state === 'unauthenticated' && (
          <div className={`bg-yellow-50 border border-yellow-200 rounded-lg p-6`}>
              <p className="text-yellow-800 mb-3">
                  Please login to download the report.
              </p>

              <button
                  onClick={handleLogin}
                  className={`bg-yellow-600 text-white px-4 py-2 rounded-lg hover:bg-yellow-700 transition`}
              >
                  Login
              </button>
          </div>
        )}

        {state === 'not-found' && (
            <div className="bg-blue-50 border border-blue-200 rounded-lg p-6">
                <p className="text-blue-800">
                    Report is not available.
                </p>
            </div>
        )}

        {state === 'error' && (
          <div className="mt-4 p-4 bg-red-100 text-red-700 rounded">
            {error}
          </div>
        )}
      </div>
    </div>
  );
};

export default ReportPage;