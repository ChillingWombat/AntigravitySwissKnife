export type OnboardingFixtureButton = {
  label: string;
  kind: "accept" | "other";
};

export const onboardingFixtureButtons: readonly OnboardingFixtureButton[] = [
  { label: "Allow usage data", kind: "other" },
  { label: "Accept and continue", kind: "accept" },
  { label: "Cancel", kind: "other" },
];
