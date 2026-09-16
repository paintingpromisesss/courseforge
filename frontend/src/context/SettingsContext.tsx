import { createContext, useContext, useState } from 'react';

export type SettingsTab = 'general' | 'courses' | 'runners' | 'ai' | 'mcp';

interface SettingsContextValue {
  isSettingsOpen: boolean;
  openSettings: (tab?: SettingsTab) => void;
  closeSettings: () => void;
  activeTab: SettingsTab;
  setActiveTab: (tab: SettingsTab) => void;
}

const SettingsContext = createContext<SettingsContextValue>({
  isSettingsOpen: false,
  openSettings: () => {},
  closeSettings: () => {},
  activeTab: 'general',
  setActiveTab: () => {},
});

export function SettingsProvider({ children }: { children: React.ReactNode }) {
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<SettingsTab>('general');

  const openSettings = (tab?: SettingsTab) => {
    if (tab) setActiveTab(tab);
    setIsSettingsOpen(true);
  };

  const closeSettings = () => setIsSettingsOpen(false);

  return (
    <SettingsContext.Provider
      value={{
        isSettingsOpen,
        openSettings,
        closeSettings,
        activeTab,
        setActiveTab,
      }}
    >
      {children}
    </SettingsContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export const useSettings = () => useContext(SettingsContext);
