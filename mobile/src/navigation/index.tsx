// React Navigation root setup for WizLetter mobile
// Maps to React Router routes in web/src/App.tsx (no admin screens — web-only)

import React from "react";
import { View, Text, StyleSheet } from "react-native";
import { NavigationContainer } from "@react-navigation/native";
import { createNativeStackNavigator } from "@react-navigation/native-stack";
import { LandingScreen } from "../screens/LandingScreen";
import { LatestScreen } from "../screens/LatestScreen";

// Placeholder for screens not yet implemented — replaced by other workers
function PlaceholderScreen({ route }: { route: { name: string } }) {
  return (
    <View style={placeholder.container}>
      <Text style={placeholder.text}>{route.name} (coming soon)</Text>
    </View>
  );
}
const placeholder = StyleSheet.create({
  container: { flex: 1, alignItems: "center", justifyContent: "center", backgroundColor: "#fdf9ee" },
  text: { fontSize: 16, color: "#231815" },
});

// These will be replaced by the real implementations once worker-4/5 complete them
let AllNewsScreen: React.ComponentType<any> = PlaceholderScreen;
let TopicDetailScreen: React.ComponentType<any> = PlaceholderScreen;
let MyPageScreen: React.ComponentType<any> = PlaceholderScreen;
let WithdrawalScreen: React.ComponentType<any> = PlaceholderScreen;
let TermsConsentScreen: React.ComponentType<any> = PlaceholderScreen;
let EmailVerificationScreen: React.ComponentType<any> = PlaceholderScreen;
let AdminScreen: React.ComponentType<any> = PlaceholderScreen;
let AdminCoinsScreen: React.ComponentType<any> = PlaceholderScreen;
let AdminWithdrawalsScreen: React.ComponentType<any> = PlaceholderScreen;
let AdminTermsScreen: React.ComponentType<any> = PlaceholderScreen;
let AdminPushScreen: React.ComponentType<any> = PlaceholderScreen;

try { AllNewsScreen = require("../screens/AllNewsScreen").AllNewsScreen; } catch {}
try { TopicDetailScreen = require("../screens/TopicDetailScreen").TopicDetailScreen; } catch {}
try { MyPageScreen = require("../screens/MyPageScreen").MyPageScreen; } catch {}
try { WithdrawalScreen = require("../screens/WithdrawalScreen").WithdrawalScreen; } catch {}
try { TermsConsentScreen = require("../screens/TermsConsentScreen").TermsConsentScreen; } catch {}
try { EmailVerificationScreen = require("../screens/EmailVerificationScreen").EmailVerificationScreen; } catch {}
try { AdminScreen = require("../screens/AdminScreen").AdminScreen; } catch {}
try { AdminCoinsScreen = require("../screens/AdminCoinsScreen").AdminCoinsScreen; } catch {}
try { AdminWithdrawalsScreen = require("../screens/AdminWithdrawalsScreen").AdminWithdrawalsScreen; } catch {}
try { AdminTermsScreen = require("../screens/AdminTermsScreen").AdminTermsScreen; } catch {}
try { AdminPushScreen = require("../screens/AdminPushScreen").AdminPushScreen; } catch {}

export type RootStackParamList = {
  Landing: undefined;
  Latest: undefined;
  AllNews: undefined;
  TopicDetail: { id: string };
  MyPage: { tab?: string } | undefined;
  Withdrawal: undefined;
  TermsConsent: { signupKey: string };
  EmailVerification: undefined;
  Admin: undefined;
  AdminCoins: undefined;
  AdminWithdrawals: undefined;
  AdminTerms: undefined;
  AdminPush: undefined;
};

const Stack = createNativeStackNavigator<RootStackParamList>();

export function RootNavigator() {
  return (
    <NavigationContainer>
      <Stack.Navigator
        initialRouteName="Landing"
        screenOptions={{ headerShown: false }}
      >
        <Stack.Screen name="Landing" component={LandingScreen} />
        <Stack.Screen name="Latest" component={LatestScreen} />
        <Stack.Screen name="AllNews" component={AllNewsScreen} />
        <Stack.Screen
          name="TopicDetail"
          component={TopicDetailScreen}
          options={{ headerShown: true, title: "" }}
        />
        <Stack.Screen
          name="MyPage"
          component={MyPageScreen}
          options={{ headerShown: true, title: "마이페이지" }}
        />
        <Stack.Screen
          name="Withdrawal"
          component={WithdrawalScreen}
          options={{ headerShown: true, title: "포인트 출금" }}
        />
        <Stack.Screen
          name="TermsConsent"
          component={TermsConsentScreen}
          options={{ headerShown: true, title: "약관 동의" }}
        />
        <Stack.Screen
          name="EmailVerification"
          component={EmailVerificationScreen}
          options={{ headerShown: true, title: "이메일 인증" }}
        />
        <Stack.Screen
          name="Admin"
          component={AdminScreen}
          options={{ headerShown: true, title: "관리자 페이지" }}
        />
        <Stack.Screen
          name="AdminCoins"
          component={AdminCoinsScreen}
          options={{ headerShown: true, title: "포인트 수정" }}
        />
        <Stack.Screen
          name="AdminWithdrawals"
          component={AdminWithdrawalsScreen}
          options={{ headerShown: true, title: "출금 관리" }}
        />
        <Stack.Screen
          name="AdminTerms"
          component={AdminTermsScreen}
          options={{ headerShown: true, title: "이용 약관 관리" }}
        />
        <Stack.Screen
          name="AdminPush"
          component={AdminPushScreen}
          options={{ headerShown: true, title: "푸시 알림 관리" }}
        />
      </Stack.Navigator>
    </NavigationContainer>
  );
}
