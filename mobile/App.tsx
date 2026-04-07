import { StatusBar } from 'expo-status-bar';
import { AuthProvider } from './src/contexts/auth-context';
import { RootNavigator } from './src/navigation';

export default function App() {
  return (
    <AuthProvider>
      <RootNavigator />
      <StatusBar style="auto" />
    </AuthProvider>
  );
}
