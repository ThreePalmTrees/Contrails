import { useState } from "react";
import { ChevronRight, FolderPlus, Eye, Play, X } from "lucide-react";
import { ContrailsIcon } from "./ContrailsIcon";

interface Props {
  onComplete: () => void;
}

const steps = [
  {
    icon: <ContrailsIcon style={{ width: '28px' }} />,
    title: "Welcome to Contrails",
    description:
      "Contrails preserves your AI coding agent chat history as readable markdown files, so you never lose context.",
  },
  {
    icon: <FolderPlus size={28} />,
    title: "Add a Project",
    description:
      "Click the + button in the sidebar to connect a VS Code workspace. Contrails will find your chat sessions automatically.",
  },
  {
    icon: <Eye size={28} />,
    title: "Automatic Watching",
    description:
      "New chat sessions are processed in real-time as you work. Contrails runs quietly in the background.",
  },
  {
    icon: <Play size={28} />,
    title: "Process Existing Chats",
    description:
      'After adding a project, use the "Process All Now" button to import any existing chat sessions.',
  },
];

export function OnboardingTour({ onComplete }: Props) {
  const [currentStep, setCurrentStep] = useState(0);

  const isLastStep = currentStep === steps.length - 1;
  const step = steps[currentStep];

  return (
    <div className="onboarding-overlay">
      <div className="onboarding-card">
        <button className="onboarding-skip" onClick={onComplete} title="Skip tour">
          <X size={16} />
        </button>

        <div className="onboarding-icon">{step.icon}</div>
        <h2 className="onboarding-title">{step.title}</h2>
        <p className="onboarding-description">{step.description}</p>

        <div className="onboarding-footer">
          <div className="onboarding-dots">
            {steps.map((_, i) => (
              <div
                key={i}
                className={`onboarding-dot ${i === currentStep ? "active" : ""}`}
              />
            ))}
          </div>

          <button
            className="btn btn-primary"
            onClick={() => {
              if (isLastStep) {
                onComplete();
              } else {
                setCurrentStep((s) => s + 1);
              }
            }}
          >
            {isLastStep ? (
              "Get Started"
            ) : (
              <>
                Next <ChevronRight size={14} />
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
