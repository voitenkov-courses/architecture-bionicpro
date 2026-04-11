import React from 'react';
import ReportPage from './components/ReportPage';

// Перешел на BFF, убрал Keycloak, токенов нет на frontend, аутентификация через cookie

const App: React.FC = () => {
  return (
    <div className="App">
      <ReportPage />
    </div>
  );
};

export default App;